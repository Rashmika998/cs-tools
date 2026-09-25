// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
)

// This file implements the three endpoints added for WSO2's internal UMT
// (product-update-release-management) tool: case lookup by number, case
// label/tag, and a composite "conclude" operation. See CLAUDE.md's "Adding a
// new endpoint" procedure for the conventions followed here.

// caseFieldFilter mirrors entity-service's domain.CaseFieldFilter -- a single
// "field op values" predicate in a case search filter expression.
type caseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

// searchCasesByNumberRequest builds the entity-service POST /cases/search body
// for an exact-match lookup by case number, mirroring domain.SearchCasesRequest.
type searchCasesByNumberRequest struct {
	Filters struct {
		Filters []caseFieldFilter `json:"filters"`
	} `json:"filters"`
}

// LookupCase handles GET /cases/lookup?number={caseNumber}. UMT has a case
// number (e.g. CS0012345) and needs this platform's own case UUID -- never
// ServiceNow's internal sys_id, which the entity service's case model never
// exposes. This proxies to entity-service's POST /cases/search with a
// confirmed exact-match field filter on "number". Following this service's
// raw-[]byte-passthrough philosophy, the entity-service search response
// (an object whose "cases" array holds 0 or 1 matching case for an exact
// number match) is forwarded as-is rather than reshaped into a single object.
func (h *CaseHandler) LookupCase(w http.ResponseWriter, r *http.Request) {
	number := strings.TrimSpace(r.URL.Query().Get("number"))
	if number == "" {
		writeError(w, http.StatusBadRequest, ErrMsgNumberRequired)
		return
	}

	var searchReq searchCasesByNumberRequest
	searchReq.Filters.Filters = []caseFieldFilter{
		{Field: "number", Op: "eq", Values: []string{number}},
	}
	body, err := json.Marshal(searchReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal case lookup filter failed", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "number", number, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to look up case.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// addCaseLabelRequest is the caller-facing request body for AddCaseLabel: only
// "label" is ever accepted from the caller. actorEmail is never a caller
// input -- see CaseHandler.umtActorEmail's doc comment.
type addCaseLabelRequest struct {
	Label string `json:"label"`
}

// addCaseTagUpstreamRequest is the entity-service POST /cases/{id}/tags body
// this handler builds, mirroring domain.AddCaseTagRequest. ActorEmail is
// always this service's own configured M2M identity.
type addCaseTagUpstreamRequest struct {
	Label      string `json:"label"`
	ActorEmail string `json:"actorEmail"`
}

// AddCaseLabel handles POST /cases/{id}/tags. The caller supplies only
// "label"; this handler supplies entity-service's actorEmail field itself,
// from this service's own configured trusted M2M identity
// (CaseHandler.umtActorEmail) -- it is never taken from the caller, since
// that would let any caller claim to be any user. If umtActorEmail is unset
// or not on entity-service's M2M_TRUSTED_ACTOR_EMAILS allowlist, entity-service
// rejects the call with 403, which is surfaced normally rather than special-cased
// here.
func (h *CaseHandler) AddCaseLabel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(raw) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var req addCaseLabelRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		writeError(w, http.StatusBadRequest, ErrMsgLabelRequired)
		return
	}

	upstreamBody, err := json.Marshal(addCaseTagUpstreamRequest{
		Label:      req.Label,
		ActorEmail: h.umtActorEmail,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal add case tag body failed", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.AddCaseTag(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCaseTag failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to add case label.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// concludeCaseRequest is the caller-facing request body for ConcludeCase.
// UpdateLevel is optional; when present it is folded into the comment content
// sent upstream (see ConcludeCase's doc comment) rather than forwarded to
// PatchCase, since entity-service's markFixIssued update cannot be combined
// with any other field.
type concludeCaseRequest struct {
	Comment     string `json:"comment"`
	UpdateLevel string `json:"updateLevel,omitempty"`
}

// legOutcome reports one leg of a composite operation's outcome independently.
// Result carries the raw upstream response on success (omitted on failure);
// Error carries a short, log-safe summary on failure (omitted on success) --
// never the raw upstream error body, per this service's error-message
// convention (see response.go).
type legOutcome struct {
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// concludeCaseResponse is the response body for POST /cases/{id}/conclude.
type concludeCaseResponse struct {
	MarkFixIssued legOutcome `json:"markFixIssued"`
	Comment       legOutcome `json:"comment"`
}

// markFixIssuedPatchBody is the entity-service PATCH /cases/{id} body sent by
// ConcludeCase's first leg.
type markFixIssuedPatchBody struct {
	MarkFixIssued bool `json:"markFixIssued"`
}

// concludeCommentBody is the entity-service POST /cases/{id}/comments body
// sent by ConcludeCase's second leg, mirroring
// domain.CreateCaseCommentRequest. ActorEmail is always this service's own
// configured M2M identity -- ConcludeCase builds and sends this body
// directly to the entity client, it does not go through
// CaseHandler.CreateCaseComment's own actorEmail-injection logic, so this
// leg has to inject it itself, the same way AddCaseLabel does.
type concludeCommentBody struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	ActorEmail string `json:"actorEmail"`
}

// ConcludeCase handles POST /cases/{id}/conclude, the first composite endpoint
// in this service. It calls entity-service twice:
//
//  1. PatchCase with {"markFixIssued": true} -- expected to succeed over
//     M2M (Postgres-backed, first-write-wins, no forwarded identity required).
//  2. CreateCaseComment with the caller's comment -- expected to succeed
//     over M2M too, now that entity-service accepts a caller-supplied
//     actorEmail checked against its M2M_TRUSTED_ACTOR_EMAILS allowlist (see
//     concludeCommentBody's own doc comment): this leg injects
//     h.umtActorEmail into its own request body directly, the same way
//     CaseHandler.CreateCaseComment and AddCaseLabel do for their own
//     endpoints.
//
// Both legs are always attempted, regardless of the other's outcome, and the
// response always reports both independently rather than letting the
// comment leg's expected failure hide the fix-issued mark's success (or vice
// versa) -- a future ServiceNow-side fix or dual-write catch-up could make
// the comment leg succeed later, so it is never skipped. This handler returns
// 200 whenever the request itself is well-formed (a well-formed request whose
// legs both fail is still a 200 reporting two failures -- the leg outcomes
// are the payload, not an HTTP-level error); a malformed request (missing
// comment, invalid case UUID, oversized/invalid body) still gets an ordinary
// 4xx.
func (h *CaseHandler) ConcludeCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(raw) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var req concludeCaseRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if strings.TrimSpace(req.Comment) == "" {
		writeError(w, http.StatusBadRequest, ErrMsgCommentRequired)
		return
	}

	resp := concludeCaseResponse{
		MarkFixIssued: h.concludeMarkFixIssued(r, id),
		Comment:       h.concludeAddComment(r, id, req),
	}

	respBody, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal conclude case response failed", "caseID", id, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, respBody)
}

// concludeMarkFixIssued runs ConcludeCase's first leg.
func (h *CaseHandler) concludeMarkFixIssued(r *http.Request, caseID string) legOutcome {
	body, err := json.Marshal(markFixIssuedPatchBody{MarkFixIssued: true})
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal markFixIssued patch body failed", "caseID", caseID, "err", err)
		return legOutcome{Success: false, Error: ErrMsgInternal}
	}

	result, err := h.entity.PatchCase(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase (markFixIssued) failed", "caseID", caseID, "err", summarizeErr(err))
		return legOutcome{Success: false, Error: legErrorMessage(err)}
	}
	return legOutcome{Success: true, Result: json.RawMessage(result)}
}

// concludeAddComment runs ConcludeCase's second leg. UpdateLevel, when
// present, is folded into the comment content -- entity-service's comment
// body has no separate field for it.
func (h *CaseHandler) concludeAddComment(r *http.Request, caseID string, req concludeCaseRequest) legOutcome {
	content := req.Comment
	if req.UpdateLevel != "" {
		content = fmt.Sprintf("[Update Level: %s] %s", req.UpdateLevel, req.Comment)
	}

	body, err := json.Marshal(concludeCommentBody{Type: "comment", Content: content, ActorEmail: h.umtActorEmail})
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal conclude comment body failed", "caseID", caseID, "err", err)
		return legOutcome{Success: false, Error: ErrMsgInternal}
	}

	result, err := h.entity.CreateCaseComment(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment (conclude) failed", "caseID", caseID, "err", summarizeErr(err))
		return legOutcome{Success: false, Error: legErrorMessage(err)}
	}
	return legOutcome{Success: true, Result: json.RawMessage(result)}
}

// legErrorMessage maps an upstream error to a short, caller-safe summary for
// a composite leg outcome, following the same status-code mapping as
// mapUpstreamError but returning a string instead of writing an HTTP
// response -- never the raw upstream error body.
func legErrorMessage(err error) string {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return ErrMsgUnauthorized
		case http.StatusForbidden:
			return ErrMsgForbidden
		case http.StatusNotFound:
			return ErrMsgNotFound
		case http.StatusBadRequest:
			return ErrMsgBadRequest
		case http.StatusConflict, http.StatusUnprocessableEntity:
			return fmt.Sprintf("Upstream returned status %d.", apiErr.StatusCode)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return "Upstream service unavailable."
		default:
			return ErrMsgInternal
		}
	}
	return ErrMsgInternal
}
