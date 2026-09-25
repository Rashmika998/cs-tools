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

package service

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SLAEngineService is the CSM-native SLA clock engine: it registers,
// completes, pauses and resumes source='CSM' "sla" rows (migration 000088)
// in reaction to case-lifecycle events, sourcing real durations from the
// ServiceNow-synced sla_policy table (via slaPolicyResolver) instead of the
// old, deleted sla_clocks design's hardcoded severity->duration map (see
// git history for internal/service/sla_policy.go before commit
// 116d43522). Every method here is called directly, in-process, from
// snCaseService's own case-lifecycle hooks (sn_case_service.go) -- never
// over HTTP, and never gated on Event Hub/publisher being configured, same
// reasoning the old design's applyResponseSLAOnComment/
// applyCaseStateSLAEffects gave for being independent of s.publisher: a
// deployment with no Event Hub configured must not lose SLA tracking as a
// side effect.
//
// Every method is best-effort: a failure here is logged and never returned
// to the caller as an error, because none of this may ever fail case
// creation, comment creation, or a state-changing PATCH -- the case/comment
// mutation itself has already succeeded (in ServiceNow) by the time any of
// these run, and treating an SLA-tracking hiccup as a failed case mutation
// would be strictly worse than tracking nothing.
type SLAEngineService interface {
	// RegisterCaseClocks resolves and registers a new source='CSM' "sla"
	// row for every clock type severity applies to (see
	// slaApplicableClockTypes) -- called once, from CreateCase, right after
	// a case is created. projectID may be empty (see resolveCasePlan's own
	// doc comment); severity may be nil (a real, if rare, production state
	// -- see domain.CaseView.Severity's own doc comment on confirmed null
	// rates), in which case this logs and returns without registering
	// anything, same as the old design's "severity not in map" handling.
	RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string)
	// CompleteResponseClock marks the case's CSM-authored "response" clock
	// ACHIEVED -- called from CreateCaseComment when the new comment
	// qualifies as the case's first substantive support-engineer reply (see
	// sn_case_service.go's applyResponseSLAOnComment-equivalent hook for the
	// exact qualification check, ported from the old design).
	CompleteResponseClock(ctx context.Context, caseID string)
	// ApplyCaseStateEffects pauses/resumes/completes the case's
	// CSM-authored "workaround"/"resolution" clocks in reaction to a
	// state-changing PATCH -- see the old design's applyCaseStateSLAEffects
	// for the exact per-state behavior this ports (unchanged, including its
	// documented workaround-completion gap).
	ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState)
}

type slaEngineService struct {
	resolver   *slaPolicyResolver
	repo       repository.SLAEngineRepository
	projectSvc ProjectService
}

// NewSLAEngineService constructs an SLAEngineService backed by the given
// repository. projectSvc backs resolveCasePlan's own project lookup (see
// its doc comment) and may be nil -- every call site already tolerates a
// nil projectSvc by falling back to slaPlanOpenSource.
func NewSLAEngineService(repo repository.SLAEngineRepository, projectSvc ProjectService) SLAEngineService {
	return &slaEngineService{resolver: newSLAPolicyResolver(repo), repo: repo, projectSvc: projectSvc}
}

// RegisterCaseClocks implements SLAEngineService.
func (s *slaEngineService) RegisterCaseClocks(ctx context.Context, caseID string, severity *domain.CaseSeverity, projectID string) {
	if severity == nil {
		slog.InfoContext(ctx, "sla engine: not registering clocks, case has no severity", "caseId", caseID)
		return
	}
	clockTypes, ok := slaApplicableClockTypes[*severity]
	if !ok || len(clockTypes) == 0 {
		slog.WarnContext(ctx, "sla engine: not registering clocks, no applicable clock types for severity", "caseId", caseID, "severity", *severity)
		return
	}

	plan := resolveCasePlan(ctx, s.projectSvc, projectID)
	for _, clockType := range clockTypes {
		policy, ok := s.resolver.resolve(ctx, *severity, clockType, plan)
		if !ok {
			// resolve already logged why -- registering nothing for this
			// clock type is the same "no fallback duration" behavior the
			// old slaDurations map's absent map entries had.
			continue
		}
		registered, err := s.repo.RegisterClock(ctx, caseID, policy)
		if err != nil {
			slog.ErrorContext(ctx, "sla engine: register clock failed", "caseId", caseID, "clockType", clockType, "err", err)
			continue
		}
		if !registered {
			slog.InfoContext(ctx, "sla engine: clock already registered, skipped", "caseId", caseID, "clockType", clockType)
		}
	}
}

// CompleteResponseClock implements SLAEngineService.
func (s *slaEngineService) CompleteResponseClock(ctx context.Context, caseID string) {
	if _, err := s.repo.CompleteClock(ctx, caseID, slaClockTypeTarget[slaClockTypeResponse]); err != nil {
		slog.ErrorContext(ctx, "sla engine: complete response clock failed", "caseId", caseID, "err", err)
	}
}

// ApplyCaseStateEffects implements SLAEngineService.
//
//   - CaseStateAwaitingInfo/CaseStateSolutionProposed: pause both
//     workaround and resolution -- the case is waiting on the customer, not
//     actively being worked.
//   - CaseStateClosed: resume then complete resolution (claims 100%, same
//     as CompleteResponseClock does for "response"); workaround is only
//     paused, never completed -- ported unchanged from the old, deleted
//     design's own documented gap: there is no "workaround provided"
//     completion signal wired into this hook (see this engine's delivering
//     task's own final report for a note that a WorkaroundProvided field
//     now exists on domain.UpdateCaseRequest/CaseView, added by an
//     unrelated commit after the old design was written -- wiring it in
//     here is explicitly out of scope for this change).
//   - Anything else: resume both -- the case is active again.
func (s *slaEngineService) ApplyCaseStateEffects(ctx context.Context, caseID string, state domain.CaseState) {
	workaroundTarget := slaClockTypeTarget[slaClockTypeWorkaround]
	resolutionTarget := slaClockTypeTarget[slaClockTypeResolution]

	switch state {
	case domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed:
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: pause workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: pause resolution clock failed", "caseId", caseID, "err", err)
		}
	case domain.CaseStateClosed:
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume resolution clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.CompleteClock(ctx, caseID, resolutionTarget); err != nil {
			slog.ErrorContext(ctx, "sla engine: complete resolution clock failed", "caseId", caseID, "err", err)
		}
		// workaround: paused, not completed -- see this method's own doc
		// comment above.
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, true); err != nil {
			slog.ErrorContext(ctx, "sla engine: pause workaround clock failed", "caseId", caseID, "err", err)
		}
	default:
		if _, err := s.repo.SetPaused(ctx, caseID, workaroundTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume workaround clock failed", "caseId", caseID, "err", err)
		}
		if _, err := s.repo.SetPaused(ctx, caseID, resolutionTarget, false); err != nil {
			slog.ErrorContext(ctx, "sla engine: resume resolution clock failed", "caseId", caseID, "err", err)
		}
	}
}
