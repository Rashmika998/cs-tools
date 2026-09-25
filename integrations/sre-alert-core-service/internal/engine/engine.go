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

// Package engine reads, normalizes, dedups by fingerprint, and forwards one alert id at a time to CSM and Chat, folding repeats into an existing incident.
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

// alertReader is the narrow subset of *store.AlertRepo's methods the engine actually needs, letting tests substitute a fake without depending on the full repo type.
type alertReader interface {
	Get(ctx context.Context, id string) (model.Alert, error)
}

// incidentStore is the narrow subset of *store.IncidentRepo's methods the engine actually needs, letting tests substitute a fake without depending on the full repo type.
type incidentStore interface {
	FindByFingerprint(ctx context.Context, fingerprint string) (model.Incident, bool, error)
	Upsert(ctx context.Context, alertID string, a model.Alert, severityNum int) (model.Incident, bool, error)
	RecordAlertID(ctx context.Context, existing model.Incident, alertID string) error
	RecordCSMIncident(ctx context.Context, fingerprint, incidentID, incidentNumber string) error
	RecordCSMAttemptFailure(ctx context.Context, fingerprint string, attempts, maxAttempts int, permanent bool) error
	SyncStatus(ctx context.Context, fingerprint, status string) error
	AppendWorkNote(ctx context.Context, existing model.Incident, note string) error
	MarkNotified(ctx context.Context, fingerprint string) error
	ListPending(ctx context.Context) ([]model.Incident, error)
}

// notifier is the narrow subset of *notify.Notifier's methods the engine actually needs, letting tests substitute a fake without depending on the full notifier type.
type notifier interface {
	NotifyCSM(ctx context.Context, inc model.Incident) (incidentID, incidentNumber string, ok bool, permanent bool)
	NotifyChat(ctx context.Context, inc model.Incident) (ok bool)
	PushWorkNote(ctx context.Context, incidentID, note string) error
	IncidentState(ctx context.Context, incidentNumber string) (open bool, found bool, err error)
}

// Engine wires one repo per entity plus the notifier together; each dependency field is named for exactly the data or system it touches during processing.
type Engine struct {
	logger    *slog.Logger
	alerts    alertReader
	incidents incidentStore
	notifier  notifier
	defaults  model.Defaults
	// maxCSMAttempts bounds how many failed CreateIncident attempts an incident absorbs before it's marked permanently failed and dropped from RetrySweep -- see store.IncidentRepo.RecordCSMAttemptFailure.
	maxCSMAttempts int
	// locks hands out one mutex per fingerprint, replacing a single global lock: deliverAndPersist for distinct incidents never serializes behind each other, and the two callers that can race the same fingerprint (Handle and RetrySweep) always re-read the row inside the lock rather than trusting a possibly-stale snapshot.
	locks *fpLocks
}

// New wires the engine's collaborators together — typically the real *store.AlertRepo, *store.IncidentRepo, and *notify.Notifier — plus the alert default values to apply and the CSM delivery attempt cap.
func New(logger *slog.Logger, alerts alertReader, incidents incidentStore, n notifier, defaults model.Defaults, maxCSMAttempts int) *Engine {
	return &Engine{logger: logger, alerts: alerts, incidents: incidents, notifier: n, defaults: defaults, maxCSMAttempts: maxCSMAttempts, locks: newFPLocks()}
}

// Outcome tells the poller what happened to an alert id, determining whether the poller's durable cursor may safely advance past that id or must retry it.
type Outcome int

const (
	// Processed means the alert was folded into an incident, or already had been; the poller may safely advance its cursor past this alert id.
	Processed Outcome = iota
	// Retry means processing hit a transient condition, such as the row not yet being visible or a write failing; the poller retries this id next cycle.
	Retry
	// Failed means the row exists but can't be processed, for example due to malformed JSON; it will never resolve on retry, so the poller skips past it.
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

// Process handles one stored alert id end to end, composing Prepare and Handle; errors are logged internally rather than returned, since the caller only needs the Outcome.
func (e *Engine) Process(ctx context.Context, alertID string) Outcome {
	alert, _, outcome, ready := e.Prepare(ctx, alertID)
	if !ready {
		return outcome
	}
	return e.Handle(ctx, alertID, alert)
}

// Prepare reads and normalizes one alert, returning its fingerprint for sharding; ready is false when unprocessable, with outcome then Retry or Failed, and true means pass alert to Handle.
func (e *Engine) Prepare(ctx context.Context, alertID string) (alert model.Alert, fingerprint string, outcome Outcome, ready bool) {
	alert, err := e.alerts.Get(ctx, alertID)
	if err != nil {
		if errors.Is(err, store.ErrMalformedAlert) {
			e.logger.Error("alert unprocessable, skipping", "alert_id", alertID, "error", err)
			return model.Alert{}, "", Failed, false
		}
		e.logger.Info("alert read failed or not visible yet, will retry", "alert_id", alertID, "error", err)
		return model.Alert{}, "", Retry, false
	}
	e.defaults.Apply(&alert)
	fingerprint = model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)
	return alert, fingerprint, Processed, true
}

// Handle folds a normalized alert into its incident and forwards notifications; safe to call concurrently for distinct fingerprints, but never for the same fingerprint at once.
func (e *Engine) Handle(ctx context.Context, alertID string, alert model.Alert) Outcome {
	severityNum, recognized := model.SeverityToNumeric(alert.Severity)
	if !recognized {
		// An unrecognized label silently becoming P1 Critical would page people for what might just be a typo -- log it loudly rather than defaulting silently.
		e.logger.Warn("unrecognized severity label, defaulting to critical", "alert_id", alertID, "severity", alert.Severity)
	}
	fp := model.Fingerprint(alert.Source, alert.Service, alert.MetricName, alert.Environment, alert.UniqueIdentifier)

	existing, found, err := e.incidents.FindByFingerprint(ctx, fp)
	if err != nil {
		e.logger.Warn("incident lookup failed, will retry", "alert_id", alertID, "error", err)
		return Retry
	}

	if model.IsResolving(severityNum) && !found {
		// A resolving alert with no matching incident has nothing to annotate or fold; creating one via Upsert would turn an OK/Clear alert into a spurious critical-impact incident.
		e.logger.Info("resolving alert with no matching incident, ignoring", "alert_id", alertID, "fingerprint", fp)
		return Processed
	}

	if found {
		// Open/closed state is authoritative only on CSM's side -- this service is never the party that closes an incident, so refresh the local snapshot before deciding whether to annotate or fold. See model.Incident.IsOpen's own doc comment.
		existing = e.syncIncidentState(ctx, existing)

		// Idempotency: a poll window whose cursor stalled on a later Retry re-runs every already-handled id in the same window on the next cycle (see poll.Poller's own doc comment). AlertIDs already records every alert this incident has absorbed, including ones only ever annotated (see RecordAlertID below) -- so a repeat is a no-op here rather than a second "Duplicate"/"OK" work note.
		if slices.Contains(existing.AlertIDs, alertID) {
			e.logger.Info("alert id already recorded on this incident, skipping duplicate replay", "incident_number", existing.IncidentNumber, "alert_id", alertID)
			return Processed
		}

		// A resolving alert (OK/Clear) against an existing incident is only annotated, never folded in; this must run before Upsert, or Clear's severity value would poison it.
		if model.IsResolving(severityNum) {
			return e.annotate(ctx, existing, alertID, "OK", alert)
		}

		// A duplicate alert against a still-open incident is annotated, not re-notified; against a closed incident it falls through to Upsert and folds into the same row.
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

	// Notification delivery is separate from alert processing; the alert is done once its incident row is written, and RetrySweep independently retries any pending delivery later.
	e.deliverAndPersist(ctx, inc.Fingerprint)
	return Processed
}

// annotate appends an OK/Duplicate work note to an existing incident, records alertID against it (for idempotency -- see Handle's own doc comment), and best-effort pushes the same note to CSM.
func (e *Engine) annotate(ctx context.Context, existing model.Incident, alertID, kind string, alert model.Alert) Outcome {
	note := model.BuildWorkNote(kind, alertID, alert.MetricName, alert.Source, time.Now())
	if err := e.incidents.AppendWorkNote(ctx, existing, note); err != nil {
		e.logger.Warn("work note append failed, will retry", "alert_id", alertID, "error", err)
		return Retry
	}
	if err := e.incidents.RecordAlertID(ctx, existing, alertID); err != nil {
		// Best-effort: if this fails, the worst case is a redundant note on a future retried-window replay, not a lost alert -- see Handle's idempotency check.
		e.logger.Warn("failed to record alert id for idempotency, continuing", "alert_id", alertID, "error", err)
	}
	if existing.IncidentID != "" {
		if err := e.notifier.PushWorkNote(ctx, existing.IncidentID, note); err != nil {
			// Best-effort and non-blocking, matching this service's other CSM-side annotation calls: the note is already durably recorded locally, and retrying a one-shot summary on a later scan has no natural dedup story of its own.
			e.logger.Warn("failed to push work note to csm", "incident_number", existing.IncidentNumber, "alert_id", alertID, "error", err)
		}
	}
	e.logger.Info("alert recorded on existing incident", "incident_number", existing.IncidentNumber, "alert_id", alertID, "kind", kind)
	return Processed
}

// syncIncidentState refreshes inc's local Status from CSM's own incident state when a real CSM incident exists, so model.Incident.IsOpen reflects reality rather than a value only this service ever wrote. A CSM lookup failure or a not-yet-existing incident leaves inc unchanged; this never blocks or fails the caller.
func (e *Engine) syncIncidentState(ctx context.Context, inc model.Incident) model.Incident {
	if !inc.CSMConfirmed {
		return inc // nothing created on CSM yet -- still open by definition, see IsOpen.
	}
	open, found, err := e.notifier.IncidentState(ctx, inc.IncidentNumber)
	if err != nil {
		e.logger.Warn("csm incident state check failed, using last known state", "incident_number", inc.IncidentNumber, "error", err)
		return inc
	}
	if !found {
		return inc
	}
	status := "closed"
	if open {
		status = "open"
	}
	if status == inc.Status {
		return inc
	}
	if err := e.incidents.SyncStatus(ctx, inc.Fingerprint, status); err != nil {
		e.logger.Warn("failed to persist synced incident status", "incident_number", inc.IncidentNumber, "error", err)
		return inc
	}
	inc.Status = status
	return inc
}

// deliverAndPersist re-reads the incident row for fingerprint under that fingerprint's own lock, attempts whichever of CSM/Chat delivery is still owed, and persists the result. Re-reading inside the lock (rather than trusting a caller-supplied snapshot) is what closes the race RetrySweep's stale ListPending snapshot used to allow: without it, Handle could confirm the incident between the sweep's list and this call, and the sweep would still see csm_confirmed=false and send CSM a second time.
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
	if inc.CSMConfirmed && inc.Notified {
		return // already fully delivered by a caller that beat us to this lock.
	}

	csmConfirmed := inc.CSMConfirmed
	if !csmConfirmed {
		id, number, ok, permanent := e.notifier.NotifyCSM(ctx, inc)
		if ok {
			if err := e.incidents.RecordCSMIncident(ctx, inc.Fingerprint, id, number); err != nil {
				// Do not treat as confirmed locally: csm_confirmed only ever flips true in the same write that persists id/number, so a partial failure here can never strand a real incident under an unconfirmed placeholder for good -- the next attempt's dedup-by-tag search (see notify.Notifier.NotifyCSM) finds this exact incident instead of creating another one.
				e.logger.Error("failed to persist csm incident, will retry", "incident_number", inc.IncidentNumber, "csm_incident_id", id, "csm_incident_number", number, "error", err)
			} else {
				csmConfirmed = true
				inc.IncidentNumber = number
				inc.IncidentID = id
			}
		} else {
			attempts := inc.CSMAttempts + 1
			if err := e.incidents.RecordCSMAttemptFailure(ctx, inc.Fingerprint, attempts, e.maxCSMAttempts, permanent); err != nil {
				e.logger.Error("failed to record csm attempt failure", "incident_number", inc.IncidentNumber, "error", err)
			} else if permanent || attempts >= e.maxCSMAttempts {
				e.logger.Error("csm permanently failed for incident, giving up", "incident_number", inc.IncidentNumber, "attempts", attempts, "permanent", permanent)
			}
		}
	}

	chatNotified := inc.Notified
	if !csmConfirmed && !chatNotified {
		// CSM didn't confirm (this attempt or a prior one), so fall back to
		// FALLBACK_CHAT_WEBHOOK_URLS so a human still sees it -- but only the first time; once
		// delivered, resending on every later CSM-retry sweep would just spam the space.
		chatNotified = e.notifier.NotifyChat(ctx, inc)
	}
	if chatNotified && !inc.Notified {
		if err := e.incidents.MarkNotified(ctx, inc.Fingerprint); err != nil {
			e.logger.Error("failed to persist notified flag", "incident_number", inc.IncidentNumber, "error", err)
		}
	}
}

// RetrySweep re-attempts delivery for every incident still owed a CSM confirmation, a Chat notification, or both (ListPending already excludes incidents CSM has permanently rejected). stillLeader is checked before each incident, so a sweep that outlives this replica's leadership stops immediately instead of racing a new leader's own sweep or cycle. It is safe to call concurrently with Process: deliverAndPersist is serialized per fingerprint and always re-reads the row before acting.
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
