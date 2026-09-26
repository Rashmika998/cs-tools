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

// Package engine dedups alerts by fingerprint and forwards them to CSM and Chat.
package engine

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"alert-core-service/internal/model"
	"alert-core-service/internal/store"
)

// alertReader lets tests fake *store.AlertRepo without the full repo type.
type alertReader interface {
	Get(ctx context.Context, id string) (model.Alert, error)
}

// incidentStore lets tests fake *store.IncidentRepo without the full repo type.
type incidentStore interface {
	FindByFingerprint(ctx context.Context, fingerprint string) (model.Incident, bool, error)
	Upsert(ctx context.Context, alertID string, a model.Alert, severityNum int) (model.Incident, bool, error)
	RecordAlertID(ctx context.Context, existing model.Incident, alertID string) error
	RecordCSMIncident(ctx context.Context, fingerprint, incidentID, incidentNumber string) error
	RecordCSMAttemptFailure(ctx context.Context, fingerprint string, attempts, maxAttempts int, permanent bool) error
	SyncStatus(ctx context.Context, fingerprint, status string, checkedAt time.Time) error
	RecordStateChecked(ctx context.Context, fingerprint string, checkedAt time.Time) error
	AppendWorkNote(ctx context.Context, existing model.Incident, note string) error
	ClearPendingNotes(ctx context.Context, fingerprint string, remaining []string) error
	MarkNotified(ctx context.Context, fingerprint string) error
	ListPending(ctx context.Context) ([]model.Incident, error)
}

// notifier lets tests fake *notify.Notifier without the full notifier type.
type notifier interface {
	NotifyCSM(ctx context.Context, inc model.Incident) (incidentID, incidentNumber string, ok bool, permanent bool)
	NotifyChat(ctx context.Context, inc model.Incident) (ok bool)
	PushWorkNote(ctx context.Context, incidentID, note string) error
	IncidentState(ctx context.Context, incidentNumber string) (open bool, found bool, err error)
}

// Engine wires one repo per entity plus the notifier together.
type Engine struct {
	logger    *slog.Logger
	alerts    alertReader
	incidents incidentStore
	notifier  notifier
	defaults  model.Defaults
	// maxCSMAttempts caps failed CreateIncident attempts before RetrySweep gives up on the incident.
	maxCSMAttempts int
	// stateCheckInterval throttles syncIncidentState's CSM round trips, so a flapping alert on a
	// confirmed incident costs at most one CSM search per interval rather than one per duplicate.
	stateCheckInterval time.Duration
	// locks is per-fingerprint so distinct incidents never serialize; racing callers re-read the row under lock.
	locks *fpLocks
}

// New wires the engine's collaborators, alert defaults, CSM attempt cap, and state-check throttle together.
func New(logger *slog.Logger, alerts alertReader, incidents incidentStore, n notifier, defaults model.Defaults, maxCSMAttempts int, stateCheckInterval time.Duration) *Engine {
	return &Engine{
		logger: logger, alerts: alerts, incidents: incidents, notifier: n, defaults: defaults,
		maxCSMAttempts: maxCSMAttempts, stateCheckInterval: stateCheckInterval, locks: newFPLocks(),
	}
}

// Outcome tells the poller whether it may advance its cursor past an alert id or must retry it.
type Outcome int

const (
	// Processed: the poller may advance its cursor past this alert id.
	Processed Outcome = iota
	// Retry: a transient failure occurred; the poller retries this id next cycle.
	Retry
	// Failed: the alert will never process successfully, so the poller skips it.
	Failed
)

func (o Outcome) String() string {
	switch o {
	case Processed:
		return "processed"
	case Retry:
		return "retry"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Process handles one stored alert id end to end via Prepare and Handle.
func (e *Engine) Process(ctx context.Context, alertID string) Outcome {
	alert, _, outcome, ready, _ := e.Prepare(ctx, alertID)
	if !ready {
		return outcome
	}
	return e.Handle(ctx, alertID, alert)
}

// Prepare reads and normalizes one alert; ready is false when unprocessable (see outcome). notFound is
// only meaningful when !ready && outcome == Retry: it's true when the row simply isn't visible yet
// (expected, temporary replication lag) and false for any other read error (e.g. Cosmos unreachable).
// The poller's gap-timeout skip must only ever fire on the former -- skipping on the latter would
// silently drop an alert during a real database outage instead of just waiting it out.
func (e *Engine) Prepare(ctx context.Context, alertID string) (alert model.Alert, fingerprint string, outcome Outcome, ready bool, notFound bool) {
	alert, err := e.alerts.Get(ctx, alertID)
	if err != nil {
		if errors.Is(err, store.ErrMalformedAlert) {
			e.logger.Error("alert unprocessable, skipping", "alert_id", alertID, "error", err)
			return model.Alert{}, "", Failed, false, false
		}
		if errors.Is(err, store.ErrAlertNotFound) {
			e.logger.Info("alert not visible yet, will retry", "alert_id", alertID, "error", err)
			return model.Alert{}, "", Retry, false, true
		}
		e.logger.Warn("alert read failed, will retry", "alert_id", alertID, "error", err)
		return model.Alert{}, "", Retry, false, false
	}
	e.defaults.Apply(&alert)
	fingerprint = model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)
	return alert, fingerprint, Processed, true, false
}

// Handle folds a normalized alert into its incident; concurrent calls must use distinct fingerprints.
func (e *Engine) Handle(ctx context.Context, alertID string, alert model.Alert) Outcome {
	severityNum, recognized := model.SeverityToNumeric(alert.Severity)
	if !recognized {
		// Log loudly: a silent default to Critical could page people for a typo.
		e.logger.Warn("unrecognized severity label, defaulting to critical", "alert_id", alertID, "severity", alert.Severity)
	}
	fp := model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)

	existing, found, err := e.incidents.FindByFingerprint(ctx, fp)
	if err != nil {
		e.logger.Warn("incident lookup failed, will retry", "alert_id", alertID, "error", err)
		return Retry
	}

	if model.IsResolving(severityNum) && !found {
		// Nothing to annotate/fold; Upsert would spuriously create an incident from an OK/Clear alert.
		e.logger.Info("resolving alert with no matching incident, ignoring", "alert_id", alertID, "fingerprint", fp)
		return Processed
	}

	if found {
		// Only CSM authoritatively closes incidents, so refresh local state before deciding.
		existing = e.syncIncidentState(ctx, existing)

		// Replayed poll windows re-run already-handled ids; AlertIDs makes that a no-op here.
		if slices.Contains(existing.AlertIDs, alertID) {
			e.logger.Info("alert id already recorded on this incident, skipping duplicate replay", "incident_number", existing.IncidentNumber, "alert_id", alertID)
			return Processed
		}

		// Must run before Upsert, or Clear's severity value would poison the incident update.
		if model.IsResolving(severityNum) {
			return e.annotate(ctx, existing, alertID, "OK", alert)
		}

		// Duplicate against an open incident is annotated; against a closed one it falls through to Upsert.
		if existing.IsOpen() {
			return e.annotate(ctx, existing, alertID, "Duplicate", alert)
		}
	}

	inc, isNew, err := e.incidents.Upsert(ctx, alertID, alert, severityNum)
	if err != nil {
		e.logger.Warn("incident upsert failed, will retry", "alert_id", alertID, "error", err)
		return Retry
	}

	if isNew {
		e.logger.Info("incident created", "incident_number", inc.IncidentNumber, "alert_id", alertID,
			"service", inc.Service, "metric_name", inc.MetricName, "severity", inc.Severity)
	} else {
		e.logger.Info("incident updated", "incident_number", inc.IncidentNumber, "alert_id", alertID,
			"alert_count", inc.AlertCount)
	}

	// Delivery is decoupled from processing; RetrySweep retries any pending delivery later.
	e.deliverAndPersist(ctx, inc.Fingerprint)
	return Processed
}

// annotate appends an OK/Duplicate work note, records alertID for idempotency, and pushes the note
// (this one plus any earlier ones still owed) to CSM via deliverAndPersist's shared retry machinery.
func (e *Engine) annotate(ctx context.Context, existing model.Incident, alertID, kind string, alert model.Alert) Outcome {
	note := model.BuildWorkNote(kind, alertID, alert.MetricName, alert.Source, time.Now())
	if err := e.incidents.AppendWorkNote(ctx, existing, note); err != nil {
		e.logger.Warn("work note append failed, will retry", "alert_id", alertID, "error", err)
		return Retry
	}
	if err := e.incidents.RecordAlertID(ctx, existing, alertID); err != nil {
		// Best-effort: failure just risks a redundant note on a future replay, not a lost alert.
		e.logger.Warn("failed to record alert id for idempotency, continuing", "alert_id", alertID, "error", err)
	}
	// Push now if CSM already has this incident; if not (CSM unconfirmed), the note stays in
	// PendingNotes and RetrySweep's ListPending will flush it once CSM confirms.
	if existing.IncidentID != "" {
		e.deliverAndPersist(ctx, existing.Fingerprint)
	}
	e.logger.Info("alert recorded on existing incident", "incident_number", existing.IncidentNumber, "alert_id", alertID, "kind", kind)
	return Processed
}

// syncIncidentState refreshes inc's local Status from CSM so IsOpen reflects reality, not stale local
// state. Throttled by stateCheckInterval: without it, a flapping alert on a confirmed incident would
// cost one CSM search per duplicate during a storm.
func (e *Engine) syncIncidentState(ctx context.Context, inc model.Incident) model.Incident {
	if !inc.CSMConfirmed {
		return inc // nothing created on CSM yet.
	}
	if e.stateCheckInterval > 0 && time.Since(inc.StateCheckedAt) < e.stateCheckInterval {
		return inc
	}
	open, found, err := e.notifier.IncidentState(ctx, inc.IncidentNumber)
	if err != nil {
		// Don't stamp StateCheckedAt: a failed check shouldn't extend the throttle window past a
		// successful one, or the next duplicate would wait a full interval for a check that never happened.
		e.logger.Warn("csm incident state check failed, using last known state", "incident_number", inc.IncidentNumber, "error", err)
		return inc
	}
	now := time.Now()
	status := inc.Status
	if found {
		status = "closed"
		if open {
			status = "open"
		}
	}
	if status == inc.Status {
		if err := e.incidents.RecordStateChecked(ctx, inc.Fingerprint, now); err != nil {
			e.logger.Warn("failed to persist state check timestamp", "incident_number", inc.IncidentNumber, "error", err)
			return inc
		}
		inc.StateCheckedAt = now
		return inc
	}
	if err := e.incidents.SyncStatus(ctx, inc.Fingerprint, status, now); err != nil {
		e.logger.Warn("failed to persist synced incident status", "incident_number", inc.IncidentNumber, "error", err)
		return inc
	}
	inc.Status = status
	inc.StateCheckedAt = now
	return inc
}

// persistTimeout bounds recording a delivery result after its external call already completed.
const persistTimeout = 5 * time.Second

// persistCtx survives ctx's cancellation, so a successful delivery still gets recorded on shutdown.
func persistCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
}

// deliverAndPersist re-reads the incident under its fingerprint's lock, delivers what's owed, and persists the result.
func (e *Engine) deliverAndPersist(ctx context.Context, fingerprint string) {
	unlock := e.locks.lock(fingerprint)
	defer unlock()

	inc, found, err := e.incidents.FindByFingerprint(ctx, fingerprint)
	if err != nil {
		e.logger.Error("delivery: failed to re-read incident", "fingerprint", fingerprint, "error", err)
		return
	}
	if !found {
		e.logger.Warn("delivery: incident vanished before delivery", "fingerprint", fingerprint)
		return
	}
	if inc.CSMConfirmed && inc.Notified && len(inc.PendingNotes) == 0 {
		return // already delivered by a caller that beat us to this lock, and no notes still owed.
	}

	csmConfirmed := inc.CSMConfirmed
	// csmSucceeded tracks CSM acceptance itself, independent of whether persisting that result below
	// succeeds: even if RecordCSMIncident fails, CSM already has this incident, so Chat must not also
	// fire and tell a human about an incident that already exists on CSM.
	csmSucceeded := csmConfirmed
	if !csmConfirmed {
		id, number, ok, permanent := e.notifier.NotifyCSM(ctx, inc)
		if ok {
			csmSucceeded = true
			pctx, cancel := persistCtx(ctx)
			err := e.incidents.RecordCSMIncident(pctx, inc.Fingerprint, id, number)
			cancel()
			if err != nil {
				// Don't mark confirmed locally; the next attempt's dedup-by-tag search will find this incident instead of duplicating it.
				e.logger.Error("failed to persist csm incident, will retry", "incident_number", inc.IncidentNumber, "csm_incident_id", id, "csm_incident_number", number, "error", err)
			} else {
				csmConfirmed = true
				inc.IncidentNumber = number
				inc.IncidentID = id
			}
		} else {
			attempts := inc.CSMAttempts + 1
			pctx, cancel := persistCtx(ctx)
			err := e.incidents.RecordCSMAttemptFailure(pctx, inc.Fingerprint, attempts, e.maxCSMAttempts, permanent)
			cancel()
			if err != nil {
				e.logger.Error("failed to record csm attempt failure", "incident_number", inc.IncidentNumber, "error", err)
			} else if permanent || attempts >= e.maxCSMAttempts {
				e.logger.Error("csm permanently failed for incident, giving up", "incident_number", inc.IncidentNumber, "attempts", attempts, "permanent", permanent)
			}
		}
	}

	if csmConfirmed && len(inc.PendingNotes) > 0 {
		inc = e.flushPendingNotes(ctx, inc)
	}

	chatNotified := inc.Notified
	if !csmSucceeded && !chatNotified {
		// Fall back to chat so a human sees it, but only the first time to avoid spamming retries.
		chatNotified = e.notifier.NotifyChat(ctx, inc)
	}
	if chatNotified && !inc.Notified {
		pctx, cancel := persistCtx(ctx)
		err := e.incidents.MarkNotified(pctx, inc.Fingerprint)
		cancel()
		if err != nil {
			e.logger.Error("failed to persist notified flag", "incident_number", inc.IncidentNumber, "error", err)
		}
	}
}

// flushPendingNotes pushes inc's PendingNotes to CSM in order, stopping at the first failure so a note
// is never skipped ahead of one still pending. Persists whatever progress was made even on partial failure.
func (e *Engine) flushPendingNotes(ctx context.Context, inc model.Incident) model.Incident {
	remaining := inc.PendingNotes
	for i, note := range inc.PendingNotes {
		if err := e.notifier.PushWorkNote(ctx, inc.IncidentID, note); err != nil {
			e.logger.Warn("failed to push work note to csm, will retry", "incident_number", inc.IncidentNumber, "error", err)
			remaining = inc.PendingNotes[i:]
			break
		}
		remaining = inc.PendingNotes[i+1:]
	}
	if len(remaining) == len(inc.PendingNotes) {
		return inc // no progress made; nothing to persist.
	}
	pctx, cancel := persistCtx(ctx)
	err := e.incidents.ClearPendingNotes(pctx, inc.Fingerprint, remaining)
	cancel()
	if err != nil {
		e.logger.Error("failed to persist pending notes progress", "incident_number", inc.IncidentNumber, "error", err)
		return inc
	}
	inc.PendingNotes = remaining
	return inc
}

// RetrySweep retries delivery for every pending incident, stopping early if leadership is lost.
func (e *Engine) RetrySweep(ctx context.Context, stillLeader func() bool) {
	pending, err := e.incidents.ListPending(ctx)
	if err != nil {
		e.logger.Error("notify retry sweep: failed to list pending incidents", "error", err)
		return
	}
	for _, inc := range pending {
		if !stillLeader() {
			e.logger.Warn("lost leadership mid-sweep, stopping", "incident_number", inc.IncidentNumber)
			return
		}
		e.deliverAndPersist(ctx, inc.Fingerprint)
	}
}
