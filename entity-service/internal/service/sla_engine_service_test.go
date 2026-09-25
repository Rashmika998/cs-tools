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
	"fmt"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingSLAEngineRepo is an in-memory repository.SLAEngineRepository
// fake that records every call, backed by the same fixed policy set
// fakePolicyLookupRepo uses (sla_policy_resolver_test.go), so
// SLAEngineService tests exercise the real resolver, not a stub.
type recordingSLAEngineRepo struct {
	fakePolicyLookupRepo
	registered []string // "workItemID|policyID"
	completed  []string // "workItemID|target"
	paused     []string // "workItemID|target|true" or "...|false"

	registerErr error
	registerOK  bool // if false, RegisterClock reports "already registered"
}

func newRecordingSLAEngineRepo() *recordingSLAEngineRepo {
	return &recordingSLAEngineRepo{fakePolicyLookupRepo: *newFakePolicyLookupRepo(), registerOK: true}
}

func (r *recordingSLAEngineRepo) RegisterClock(_ context.Context, workItemID string, policy repository.SLAPolicyRef) (bool, error) {
	if r.registerErr != nil {
		return false, r.registerErr
	}
	r.registered = append(r.registered, workItemID+"|"+policy.ID)
	return r.registerOK, nil
}

func (r *recordingSLAEngineRepo) CompleteClock(_ context.Context, workItemID, target string) (bool, error) {
	r.completed = append(r.completed, workItemID+"|"+target)
	return true, nil
}

func (r *recordingSLAEngineRepo) SetPaused(_ context.Context, workItemID, target string, paused bool) (bool, error) {
	r.paused = append(r.paused, fmt.Sprintf("%s|%s|%v", workItemID, target, paused))
	return true, nil
}

func TestSLAEngineService_RegisterCaseClocks_CatastrophicRegistersAllThree(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-1", &sev, "")

	// plan defaults to Open Source (nil projectSvc) but P0 only exists
	// under Managed Services -- the resolver's own cross-plan fallback
	// must still find all three P0 policies.
	want := []string{"case-1|p0-r-ms", "case-1|p0-w-ms", "case-1|p0-res-ms"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
	for i, w := range want {
		if repo.registered[i] != w {
			t.Errorf("registered[%d] = %q, want %q", i, repo.registered[i], w)
		}
	}
}

func TestSLAEngineService_RegisterCaseClocks_LowSeverityRegistersResponseOnly(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityLow
	svc.RegisterCaseClocks(context.Background(), "case-2", &sev, "")

	if len(repo.registered) != 1 || repo.registered[0] != "case-2|q-r-os" {
		t.Errorf("registered = %v, want exactly [case-2|q-r-os] (Query/LOW: response only, matching the old sla_clocks design's LOW-severity behavior)", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_NilSeverityRegistersNothing(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	svc.RegisterCaseClocks(context.Background(), "case-3", nil, "")

	if len(repo.registered) != 0 {
		t.Errorf("registered = %v, want none for a case with no severity", repo.registered)
	}
}

func TestSLAEngineService_RegisterCaseClocks_MissingPolicySkipsThatClockTypeOnly(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	// Remove the workaround policy so CATASTROPHIC's middle clock type has
	// nothing to resolve -- response and resolution must still register.
	delete(repo.policies, "P0 - Workaround (Managed Services)|WORKAROUND")
	svc := NewSLAEngineService(repo, nil)

	sev := domain.CaseSeverityCatastrophic
	svc.RegisterCaseClocks(context.Background(), "case-4", &sev, "")

	want := []string{"case-4|p0-r-ms", "case-4|p0-res-ms"}
	if len(repo.registered) != len(want) {
		t.Fatalf("registered = %v, want %v", repo.registered, want)
	}
}

func TestSLAEngineService_CompleteResponseClock(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	svc := NewSLAEngineService(repo, nil)

	svc.CompleteResponseClock(context.Background(), "case-5")

	if len(repo.completed) != 1 || repo.completed[0] != "case-5|RESPONSE" {
		t.Errorf("completed = %v, want [case-5|RESPONSE]", repo.completed)
	}
}

func TestSLAEngineService_ApplyCaseStateEffects(t *testing.T) {
	tests := []struct {
		name          string
		state         domain.CaseState
		wantPaused    []string
		wantCompleted []string
	}{
		{
			"awaiting info pauses both",
			domain.CaseStateAwaitingInfo,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"solution proposed pauses both",
			domain.CaseStateSolutionProposed,
			[]string{"case-6|WORKAROUND|true", "case-6|RESOLUTION|true"},
			nil,
		},
		{
			"closed resumes+completes resolution, pauses workaround only",
			domain.CaseStateClosed,
			[]string{"case-6|RESOLUTION|false", "case-6|WORKAROUND|true"},
			[]string{"case-6|RESOLUTION"},
		},
		{
			"work in progress resumes both",
			domain.CaseStateWorkInProgress,
			[]string{"case-6|WORKAROUND|false", "case-6|RESOLUTION|false"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRecordingSLAEngineRepo()
			svc := NewSLAEngineService(repo, nil)

			svc.ApplyCaseStateEffects(context.Background(), "case-6", tt.state)

			if len(repo.paused) != len(tt.wantPaused) {
				t.Fatalf("paused = %v, want %v", repo.paused, tt.wantPaused)
			}
			for i, w := range tt.wantPaused {
				if repo.paused[i] != w {
					t.Errorf("paused[%d] = %q, want %q", i, repo.paused[i], w)
				}
			}
			if len(repo.completed) != len(tt.wantCompleted) {
				t.Fatalf("completed = %v, want %v", repo.completed, tt.wantCompleted)
			}
			for i, w := range tt.wantCompleted {
				if repo.completed[i] != w {
					t.Errorf("completed[%d] = %q, want %q", i, repo.completed[i], w)
				}
			}
		})
	}
}

// TestSLAEngineService_RegisterCaseClocks_UsesResolvedPlan confirms
// RegisterCaseClocks actually threads the project-derived plan through to
// the resolver (rather than always defaulting) for a non-P0 severity,
// where the plan choice actually changes which policy matches.
func TestSLAEngineService_RegisterCaseClocks_UsesResolvedPlan(t *testing.T) {
	repo := newRecordingSLAEngineRepo()
	projectSvc := fakeProjectSvc{project: domain.ProjectDetailsView{SubscriptionType: domain.SubscriptionTypeManagedCloudSubscription}}
	svc := NewSLAEngineService(repo, projectSvc)

	sev := domain.CaseSeverityMedium // P3
	svc.RegisterCaseClocks(context.Background(), "case-7", &sev, "proj-1")

	// Only P3 - Resolution (Managed Services) is faked; response/workaround
	// for P3 aren't in the fake set at all, so only resolution registers.
	if len(repo.registered) != 1 || repo.registered[0] != "case-7|p3-res-ms" {
		t.Errorf("registered = %v, want [case-7|p3-res-ms]", repo.registered)
	}
}
