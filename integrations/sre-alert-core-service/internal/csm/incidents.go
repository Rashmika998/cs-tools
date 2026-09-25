// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package csm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CreateIncidentRequest is the request body for csm-integration-service's
// POST /incidents — a thin proxy of entity-service's own
// CreateIncidentRequest. Field names and JSON tags are copied verbatim from
// that contract; this service does not define its own incident shape.
type CreateIncidentRequest struct {
	CallerID  string  `json:"callerId"`
	Category  string  `json:"category"` // "INQUIRY" | "SERVICE_INTERRUPTION" | "SECURITY"
	ServiceID string  `json:"serviceId"`
	Impact    string  `json:"impact"`  // "HIGH" | "MEDIUM" | "LOW"
	Urgency   string  `json:"urgency"` // "HIGH" | "MEDIUM" | "LOW"
	Subject   string  `json:"subject"`
	WorkNotes *string `json:"workNotes,omitempty"`
}

// createdIncident is the subset of the response's nested "incident" object
// this service actually reads.
type createdIncident struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// createIncidentResponse is the response body for POST /incidents, decoded
// tolerantly (unknown fields ignored).
type createIncidentResponse struct {
	Message  string          `json:"message"`
	Incident createdIncident `json:"incident"`
}

// CreateIncidentResult is what CreateIncident returns on success.
type CreateIncidentResult struct {
	IncidentID     string
	IncidentNumber string
}

// CreateIncident calls POST /incidents on csm-integration-service.
func (c *Client) CreateIncident(ctx context.Context, req CreateIncidentRequest) (*CreateIncidentResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("csm: marshal CreateIncidentRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents", body)
	if err != nil {
		return nil, err
	}

	var resp createIncidentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("csm: decode CreateIncident response: %w", err)
	}
	if resp.Incident.ID == "" || resp.Incident.Number == "" {
		// A 2xx with no incident id/number is malformed, not a successful
		// create -- treat it as a failure so the caller's normal retry path
		// handles it, rather than persisting a half-populated result.
		return nil, fmt.Errorf("csm: CreateIncident response missing incident id or number")
	}

	return &CreateIncidentResult{IncidentID: resp.Incident.ID, IncidentNumber: resp.Incident.Number}, nil
}

// updateIncidentRequest is the request body for PATCH /incidents/{id}. This
// client only ever sends WorkNotes, so every other field that request shape
// accepts is left unmodeled.
type updateIncidentRequest struct {
	WorkNotes string `json:"workNotes"`
}

// UpdateIncident calls PATCH /incidents/{id} on csm-integration-service to
// push a work note onto an already-existing incident, keyed by the CSM
// incident id (not the human-readable number).
func (c *Client) UpdateIncident(ctx context.Context, incidentID, workNotes string) error {
	body, err := json.Marshal(updateIncidentRequest{WorkNotes: workNotes})
	if err != nil {
		return fmt.Errorf("csm: marshal UpdateIncident request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPatch, "/incidents/"+url.PathEscape(incidentID), body)
	return err
}

// searchIncidentsRequest is the request body for POST /incidents/search —
// only the subset this client needs (exact match on Number).
type searchIncidentsRequest struct {
	Filters    searchIncidentsFilters `json:"filters"`
	Pagination pagination             `json:"pagination"`
}

type searchIncidentsFilters struct {
	Number      string `json:"number,omitempty"`
	SearchQuery string `json:"searchQuery,omitempty"`
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type searchIncidentView struct {
	ID     *string `json:"id"`
	Number *string `json:"number"`
	State  *string `json:"state"`
}

type searchIncidentsResponse struct {
	Incidents []searchIncidentView `json:"incidents"`
	Total     int                  `json:"total"`
}

// openIncidentStates are entity-service's domain.IncidentState values (see
// entity-service/internal/domain/entity.go — a read-only reference this
// service does not import, being a separate Go module, so these literals
// are copied and must be kept in sync by hand) that represent an incident
// still being worked.
var openIncidentStates = map[string]bool{"NEW": true, "IN_PROGRESS": true, "ON_HOLD": true}

// IncidentState looks up an incident by its human-readable number via
// POST /incidents/search and reports whether it is currently open.
//
// This is the authoritative source of open/closed state: nothing about
// whether an incident is still actionable can be derived from a column only
// this service writes, since csm-integration-service (and the CSM agents
// working the incident) are the only parties that ever close one. found is
// false when the search returns no match (e.g. the incident predates this
// lookup, or number is a still-pending placeholder never actually created).
func (c *Client) IncidentState(ctx context.Context, number string) (open bool, found bool, err error) {
	req := searchIncidentsRequest{
		Filters:    searchIncidentsFilters{Number: number},
		Pagination: pagination{Limit: 1, Offset: 0},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return false, false, fmt.Errorf("csm: marshal SearchIncidentsRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents/search", body)
	if err != nil {
		return false, false, err
	}

	var resp searchIncidentsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return false, false, fmt.Errorf("csm: decode incident search response: %w", err)
	}
	if len(resp.Incidents) == 0 || resp.Incidents[0].State == nil {
		return false, false, nil
	}
	return openIncidentStates[*resp.Incidents[0].State], true, nil
}

// SearchIncidentByTag looks up an incident whose Subject carries tag (see
// notify.DedupTag) via POST /incidents/search's free-text SearchQuery.
//
// This is the pre-create dedup check: a prior CreateIncident call can
// succeed on CSM's side while its response is lost to this service (a
// timeout, a connection reset, or this service crashing right after receipt
// but before persisting the result) -- a failed call never proves nothing
// was created. Checking here first, on every CreateIncident attempt, means a
// retried delivery finds and reuses the already-created incident instead of
// creating a second one.
func (c *Client) SearchIncidentByTag(ctx context.Context, tag string) (id, number string, found bool, err error) {
	req := searchIncidentsRequest{
		Filters:    searchIncidentsFilters{SearchQuery: tag},
		Pagination: pagination{Limit: 1, Offset: 0},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", "", false, fmt.Errorf("csm: marshal SearchIncidentsRequest: %w", err)
	}

	respBody, err := c.do(ctx, http.MethodPost, "/incidents/search", body)
	if err != nil {
		return "", "", false, err
	}

	var resp searchIncidentsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", "", false, fmt.Errorf("csm: decode incident search response: %w", err)
	}
	if len(resp.Incidents) == 0 {
		return "", "", false, nil
	}
	hit := resp.Incidents[0]
	if hit.ID == nil || *hit.ID == "" || hit.Number == nil || *hit.Number == "" {
		return "", "", false, nil
	}
	return *hit.ID, *hit.Number, true, nil
}
