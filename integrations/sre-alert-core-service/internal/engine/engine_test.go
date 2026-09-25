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

package engine

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"alert-core-service/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeAlerts is a minimal in-memory alertReader.
type fakeAlerts struct {
	byID map[string]model.Alert
}

func (f *fakeAlerts) Get(_ context.Context, id string) (model.Alert, error) {
	a, ok := f.byID[id]
	if !ok {
		return model.Alert{}, context.DeadlineExceeded // any non-ErrMalformedAlert error, simulating "not visible yet"
	}
	return a, nil
}

// fakeIncidents is an in-memory incidentStore, safe for concurrent use, mirroring what
// store.IncidentRepo does against Cassandra closely enough to exercise the engine's own logic.
type fakeIncidents struct {
	mu   sync.Mutex
	byFP map[string]model.Incident
}

func newFakeIncidents() *fakeIncidents {
	return &fakeIncidents{byFP: make(map[string]model.Incident)}
}

func (f *fakeIncidents) FindByFingerprint(_ context.Context, fp string) (model.Incident, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc, ok := f.byFP[fp]
	return inc, ok, nil
}

func (f *fakeIncidents) Upsert(_ context.Context, alertID string, a model.Alert, severityNum int) (model.Incident, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fp := model.Fingerprint(a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier)
	existing, found := f.byFP[fp]
	if !found {
		impact, urgency := model.ImpactUrgency(severityNum)
		inc := model.Incident{
			Fingerprint:    fp,
			IncidentNumber: "PENDING-" + fp[:8],
			Status:         "new",
			Severity:       severityNum,
			Impact:         impact,
			Urgency:        urgency,
			Service:        a.Service,
			MetricName:     a.MetricName,
			AlertIDs:       []string{alertID},
			AlertCount:     1,
		}
		f.byFP[fp] = inc
		return inc, true, nil
	}
	for _, seen := range existing.AlertIDs {
		if seen == alertID {
			return existing, false, nil
		}
	}
	existing.AlertIDs = append(existing.AlertIDs, alertID)
	existing.AlertCount++
	if severityNum < existing.Severity {
		existing.Severity = severityNum
	}
	f.byFP[fp] = existing
	return existing, false, nil
}

func (f *fakeIncidents) RecordAlertID(_ context.Context, existing model.Incident, alertID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[existing.Fingerprint]
	for _, seen := range inc.AlertIDs {
		if seen == alertID {
			return nil
		}
	}
	inc.AlertIDs = append(inc.AlertIDs, alertID)
	f.byFP[existing.Fingerprint] = inc
	return nil
}

func (f *fakeIncidents) RecordCSMIncident(_ context.Context, fingerprint, incidentID, incidentNumber string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[fingerprint]
	inc.IncidentID = incidentID
	inc.IncidentNumber = incidentNumber
	inc.CSMConfirmed = true
	f.byFP[fingerprint] = inc
	return nil
}

func (f *fakeIncidents) RecordCSMAttemptFailure(_ context.Context, fingerprint string, attempts, maxAttempts int, permanent bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[fingerprint]
	inc.CSMAttempts = attempts
	inc.CSMPermanentlyFailed = permanent || attempts >= maxAttempts
	f.byFP[fingerprint] = inc
	return nil
}

func (f *fakeIncidents) SyncStatus(_ context.Context, fingerprint, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[fingerprint]
	inc.Status = status
	f.byFP[fingerprint] = inc
	return nil
}

func (f *fakeIncidents) AppendWorkNote(_ context.Context, existing model.Incident, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[existing.Fingerprint]
	inc.WorkNotes = append(inc.WorkNotes, note)
	f.byFP[existing.Fingerprint] = inc
	return nil
}

func (f *fakeIncidents) MarkNotified(_ context.Context, fingerprint string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc := f.byFP[fingerprint]
	inc.Notified = true
	f.byFP[fingerprint] = inc
	return nil
}

func (f *fakeIncidents) ListPending(_ context.Context) ([]model.Incident, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Incident
	for _, inc := range f.byFP {
		if !inc.CSMConfirmed && !inc.CSMPermanentlyFailed {
			out = append(out, inc)
		}
	}
	return out, nil
}

// fakeNotifier counts NotifyCSM calls so tests can assert an incident is never delivered to CSM
// twice, and lets tests control whether CSM confirms.
type fakeNotifier struct {
	csmCalls   atomic.Int32
	csmOK      bool
	csmID      string
	csmNumber  string
	chatOK     bool
	openStates map[string]bool // incident number -> open
}

func (n *fakeNotifier) NotifyCSM(_ context.Context, inc model.Incident) (string, string, bool, bool) {
	n.csmCalls.Add(1)
	if !n.csmOK {
		return "", "", false, false
	}
	return n.csmID, n.csmNumber, true, false
}

func (n *fakeNotifier) NotifyChat(_ context.Context, inc model.Incident) bool {
	return n.chatOK
}

func (n *fakeNotifier) PushWorkNote(_ context.Context, incidentID, note string) error {
	return nil
}

func (n *fakeNotifier) IncidentState(_ context.Context, incidentNumber string) (bool, bool, error) {
	open, found := n.openStates[incidentNumber]
	return open, found, nil
}

func newTestEngine(alerts map[string]model.Alert, notifier *fakeNotifier) (*Engine, *fakeIncidents) {
	incidents := newFakeIncidents()
	e := New(testLogger(), &fakeAlerts{byID: alerts}, incidents, notifier, model.Defaults{}, 3)
	return e, incidents
}

func TestHandle_ResolvingAlertWithNoMatchingIncident_Ignored(t *testing.T) {
	notifier := &fakeNotifier{}
	e, incidents := newTestEngine(nil, notifier)

	alert := model.Alert{Service: "svc", MetricName: "cpu", Severity: "ok", Source: "vendor"}
	outcome := e.Handle(context.Background(), "ALT1", alert)

	if outcome != Processed {
		t.Fatalf("outcome = %v, want Processed", outcome)
	}
	if len(incidents.byFP) != 0 {
		t.Fatalf("expected no incident to be created for a resolving alert with nothing to resolve, got %d", len(incidents.byFP))
	}
	if notifier.csmCalls.Load() != 0 {
		t.Fatalf("expected NotifyCSM never called for a resolving alert with nothing to resolve")
	}
}

func TestHandle_NewAlertCreatesAndDeliversIncident(t *testing.T) {
	notifier := &fakeNotifier{csmOK: true, csmID: "csm-1", csmNumber: "INC0000001"}
	e, incidents := newTestEngine(nil, notifier)

	alert := model.Alert{Service: "svc", MetricName: "cpu", Severity: "critical", Source: "vendor"}
	outcome := e.Handle(context.Background(), "ALT1", alert)

	if outcome != Processed {
		t.Fatalf("outcome = %v, want Processed", outcome)
	}
	if notifier.csmCalls.Load() != 1 {
		t.Fatalf("NotifyCSM calls = %d, want 1", notifier.csmCalls.Load())
	}
	fp := model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)
	inc := incidents.byFP[fp]
	if !inc.CSMConfirmed || inc.IncidentNumber != "INC0000001" || inc.IncidentID != "csm-1" {
		t.Fatalf("incident not recorded as csm-confirmed with the real id/number: %+v", inc)
	}
}

func TestHandle_DuplicateAlertOnOpenIncident_Annotates(t *testing.T) {
	notifier := &fakeNotifier{csmOK: true, csmID: "csm-1", csmNumber: "INC0000001"}
	e, incidents := newTestEngine(nil, notifier)

	alert := model.Alert{Service: "svc", MetricName: "cpu", Severity: "critical", Source: "vendor"}
	ctx := context.Background()
	e.Handle(ctx, "ALT1", alert) // creates + confirms the incident

	outcome := e.Handle(ctx, "ALT2", alert)
	if outcome != Processed {
		t.Fatalf("outcome = %v, want Processed", outcome)
	}

	fp := model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)
	inc := incidents.byFP[fp]
	if len(inc.WorkNotes) != 1 {
		t.Fatalf("expected exactly one work note recorded for the duplicate, got %d: %v", len(inc.WorkNotes), inc.WorkNotes)
	}
	if notifier.csmCalls.Load() != 1 {
		t.Fatalf("NotifyCSM calls = %d, want 1 (a duplicate on an open incident must never re-create it)", notifier.csmCalls.Load())
	}
}

// TestHandle_IdempotentReplaySkipsDuplicateNote is the case cs-tools#2016's review flagged in
// poll.Poller: a poll window whose cursor stalls on a later Retry re-runs every already-handled
// id in that window on the next cycle, including one that was only ever annotated (never
// Upserted, so Upsert's own alertID-membership check never saw it). Replaying the exact same
// alert id against the same incident must be a no-op, not a second work note.
func TestHandle_IdempotentReplaySkipsDuplicateNote(t *testing.T) {
	notifier := &fakeNotifier{csmOK: true, csmID: "csm-1", csmNumber: "INC0000001"}
	e, incidents := newTestEngine(nil, notifier)
	ctx := context.Background()

	alert := model.Alert{Service: "svc", MetricName: "cpu", Severity: "critical", Source: "vendor"}
	e.Handle(ctx, "ALT1", alert) // creates the incident
	e.Handle(ctx, "ALT2", alert) // first delivery of the duplicate note
	e.Handle(ctx, "ALT2", alert) // replayed -- must be a no-op

	fp := model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)
	inc := incidents.byFP[fp]
	if len(inc.WorkNotes) != 1 {
		t.Fatalf("expected exactly one work note after a replayed alert id, got %d: %v", len(inc.WorkNotes), inc.WorkNotes)
	}
}

// TestRetrySweepAndHandle_NeverDoubleDeliverSameIncident exercises the race cs-tools#2016's
// review flagged in the old single global notifyMu design: RetrySweep reading a stale
// ListPending snapshot while Handle concurrently confirms the same incident. deliverAndPersist
// now re-reads the row under a per-fingerprint lock, so only one of the two racing callers should
// ever actually call NotifyCSM.
func TestRetrySweepAndHandle_NeverDoubleDeliverSameIncident(t *testing.T) {
	notifier := &fakeNotifier{csmOK: true, csmID: "csm-1", csmNumber: "INC0000001"}
	e, incidents := newTestEngine(nil, notifier)
	ctx := context.Background()

	fp := model.Fingerprint("vendor", "svc", "cpu", "", "")
	incidents.byFP[fp] = model.Incident{
		Fingerprint:    fp,
		IncidentNumber: "PENDING-abc",
		Status:         "new",
		Severity:       1,
		Service:        "svc",
		MetricName:     "cpu",
		Source:         "vendor",
		AlertIDs:       []string{"ALT1"},
		AlertCount:     1,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		e.deliverAndPersist(ctx, fp)
	}()
	go func() {
		defer wg.Done()
		e.deliverAndPersist(ctx, fp)
	}()
	wg.Wait()

	if calls := notifier.csmCalls.Load(); calls != 1 {
		t.Fatalf("NotifyCSM calls = %d, want exactly 1 across two concurrent deliverAndPersist calls for the same fingerprint", calls)
	}
}
