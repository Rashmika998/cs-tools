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
	"context"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// maxOnboardingStepPages bounds how many entity-service pages
// GetProjectOnboardingSteps walks for one project. At the upstream page
// cap of 50 rows that is 2000 step rows — 500 memberships with all four
// steps recorded — far above any project seen so far, and an independent
// guard against a wrong or stuck upstream `total` turning the loop into an
// unbounded fetch. Hitting it is reported via the response's `truncated`
// flag rather than failing the request.
const maxOnboardingStepPages = 40

// onboardingStepOrder is the display order of the four onboarding steps,
// which is also the order the flow runs them in. The ledger search returns
// rows newest first, so the handler re-sorts each membership's steps here.
var onboardingStepOrder = map[string]int{
	"IDENTITY":     0,
	"DATABASE":     1,
	"EMAIL":        2,
	"REGISTRATION": 3,
}

// entityOnboardingStepClient is the slice of the entity client this handler
// needs: one typed search over the onboarding status ledger.
type entityOnboardingStepClient interface {
	SearchOnboardingSteps(ctx context.Context, req entity.OnboardingStepSearchRequest) (entity.OnboardingStepSearchResponse, error)
}

// OnboardingStepHandler serves the per-project onboarding status view. It is
// only constructed (and its route only registered) when
// CSM_MIGRATION_ONBOARDING_STATUS_ENABLED is exactly "true" — see
// cmd/server/main.go.
type OnboardingStepHandler struct {
	entity entityOnboardingStepClient
}

// NewOnboardingStepHandler constructs an OnboardingStepHandler.
func NewOnboardingStepHandler(entity entityOnboardingStepClient) *OnboardingStepHandler {
	return &OnboardingStepHandler{entity: entity}
}

// ProjectOnboardingStep is one recorded step of one membership, as the portal
// reports it. It carries only what the entity service's ledger row holds —
// no field is derived or enriched here — minus the membership-level
// identifiers that ProjectOnboardingMembership already states once.
type ProjectOnboardingStep struct {
	Step            string    `json:"step"`
	Status          string    `json:"status"`
	AttemptCount    int       `json:"attemptCount"`
	LastError       *string   `json:"lastError"`
	EventType       string    `json:"eventType"`
	EventModifiedOn time.Time `json:"eventModifiedOn"`
	UpdatedOn       time.Time `json:"updatedOn"`
}

// ProjectOnboardingMembership groups every recorded step of one Salesforce
// Project_Contact__c membership (one invited email on one project). The
// webapp matches these to the project's contact rows by lower-cased email.
type ProjectOnboardingMembership struct {
	MembershipSfID   string                  `json:"membershipSfId"`
	ContactSfID      *string                 `json:"contactSfId"`
	Email            string                  `json:"email"`
	ProjectContactID *string                 `json:"projectContactId"`
	Steps            []ProjectOnboardingStep `json:"steps"`
}

// ProjectOnboardingStepsResponse is the body of
// GET /projects/{id}/onboarding-steps.
type ProjectOnboardingStepsResponse struct {
	Memberships []ProjectOnboardingMembership `json:"memberships"`
	// Total is the number of memberships returned (not of step rows).
	Total int `json:"total"`
	// Truncated is true when the project has more step rows than
	// maxOnboardingStepPages pages could hold, so some memberships may be
	// missing or incomplete.
	Truncated bool `json:"truncated"`
}

// GetProjectOnboardingSteps handles GET /projects/{id}/onboarding-steps.
//
// It pages through the entity service's onboarding ledger filtered to the
// project (POST /onboarding-steps/search with filters.projectId), then
// regroups the flat, newest-first rows into one entry per membership with
// that membership's steps in flow order. A project with no recorded steps
// yields an empty list, not a 404.
func (h *OnboardingStepHandler) GetProjectOnboardingSteps(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	steps, truncated, err := h.fetchAllProjectSteps(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchOnboardingSteps failed", "userID", user.UserID, "projectID", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load onboarding status.")
		return
	}
	if truncated {
		slog.WarnContext(r.Context(), "onboarding step ledger truncated at page cap",
			"projectID", id, "maxPages", maxOnboardingStepPages)
	}

	memberships := groupOnboardingSteps(steps)
	writeJSONValue(w, http.StatusOK, ProjectOnboardingStepsResponse{
		Memberships: memberships,
		Total:       len(memberships),
		Truncated:   truncated,
	})
}

// fetchAllProjectSteps walks every page of the ledger search for one project.
// The upstream response has no hasMore flag, so offset+len(page) < total is
// the continuation check, bounded by maxOnboardingStepPages regardless of
// what total reports. The returned bool is true when that bound was hit.
func (h *OnboardingStepHandler) fetchAllProjectSteps(ctx context.Context, projectID string) ([]entity.OnboardingStep, bool, error) {
	var all []entity.OnboardingStep
	offset := 0
	for page := 0; page < maxOnboardingStepPages; page++ {
		resp, err := h.entity.SearchOnboardingSteps(ctx, entity.OnboardingStepSearchRequest{
			Filters:    entity.OnboardingStepFilters{ProjectID: &projectID},
			Pagination: entity.OnboardingStepPagination{Limit: entity.OnboardingStepSearchMaxLimit, Offset: offset},
		})
		if err != nil {
			return nil, false, err
		}
		all = append(all, resp.Steps...)
		offset += len(resp.Steps)
		if len(resp.Steps) == 0 || offset >= resp.Total {
			return all, false, nil
		}
	}
	return all, true, nil
}

// groupOnboardingSteps folds flat ledger rows into one entry per membership.
// Memberships are ordered by email then membership id so the response is
// stable across calls; each membership's steps follow onboardingStepOrder,
// with any step name this build does not know placed last, by name.
func groupOnboardingSteps(steps []entity.OnboardingStep) []ProjectOnboardingMembership {
	byMembership := make(map[string]*ProjectOnboardingMembership)
	for _, s := range steps {
		m, ok := byMembership[s.MembershipSfID]
		if !ok {
			m = &ProjectOnboardingMembership{
				MembershipSfID:   s.MembershipSfID,
				ContactSfID:      s.ContactSfID,
				Email:            s.Email,
				ProjectContactID: s.ProjectContactID,
				Steps:            []ProjectOnboardingStep{},
			}
			byMembership[s.MembershipSfID] = m
		}
		// The ledger holds one row per (membership, step), so these are
		// filled once each; a membership's identifiers are taken from the
		// first (newest) row seen and only back-filled when that one had
		// none, as DATABASE sets projectContactId later than IDENTITY.
		if m.ContactSfID == nil && s.ContactSfID != nil {
			m.ContactSfID = s.ContactSfID
		}
		if m.ProjectContactID == nil && s.ProjectContactID != nil {
			m.ProjectContactID = s.ProjectContactID
		}
		m.Steps = append(m.Steps, ProjectOnboardingStep{
			Step:            s.Step,
			Status:          s.Status,
			AttemptCount:    s.AttemptCount,
			LastError:       s.LastError,
			EventType:       s.EventType,
			EventModifiedOn: s.EventModifiedOn,
			UpdatedOn:       s.UpdatedOn,
		})
	}

	memberships := make([]ProjectOnboardingMembership, 0, len(byMembership))
	for _, m := range byMembership {
		sort.SliceStable(m.Steps, func(i, j int) bool {
			oi, okI := onboardingStepOrder[m.Steps[i].Step]
			oj, okJ := onboardingStepOrder[m.Steps[j].Step]
			switch {
			case okI && okJ:
				return oi < oj
			case okI != okJ:
				return okI
			default:
				return m.Steps[i].Step < m.Steps[j].Step
			}
		})
		memberships = append(memberships, *m)
	}
	sort.Slice(memberships, func(i, j int) bool {
		if memberships[i].Email != memberships[j].Email {
			return memberships[i].Email < memberships[j].Email
		}
		return memberships[i].MembershipSfID < memberships[j].MembershipSfID
	})
	return memberships
}
