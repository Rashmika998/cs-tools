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
	UpdateIncidentNumber(ctx context.Context, fingerprint, incidentNumber string) error
	AppendWorkNote(ctx context.Context, existing model.Incident, note string) error
	MarkNotified(ctx context.Context, fingerprint string) error
	MarkCSMConfirmed(ctx context.Context, fingerprint string) error
	ListPending(ctx context.Context) ([]model.Incident, error)
}

// notifier is the narrow subset of *notify.Notifier's methods the engine actually needs, letting tests substitute a fake without depending on the full notifier type.
type notifier interface {
	NotifyCSM(ctx context.Context, inc model.Incident) (incidentNumber string, ok bool)
	NotifyChat(ctx context.Context, inc model.Incident) (ok bool)
}

// Engine wires one repo per entity plus the notifier together; each dependency field is named for exactly the data or system it touches during processing.
type Engine struct {
	logger    *slog.Logger
	alerts    alertReader
	incidents incidentStore
	notifier  notifier
	defaults  model.Defaults
}

// New wires the engine's collaborators together — typically the real *store.AlertRepo, *store.IncidentRepo, and *notify.Notifier — plus the alert default values to apply.
func New(logger *slog.Logger, alerts alertReader, incidents incidentStore, n notifier, defaults model.Defaults) *Engine {
	return &Engine{logger: logger, alerts: alerts, incidents: incidents, notifier: n, defaults: defaults}
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
	severityNum := model.SeverityToNumeric(alert.Severity)
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
		// A resolving alert (OK/Clear) against an existing incident is only annotated, never folded in; this must run before Upsert, or Clear's severity value would poison it.
		if model.IsResolving(severityNum) {
			note := model.BuildWorkNote("OK", alertID, alert.MetricName, alert.Source, time.Now())
			if err := e.incidents.AppendWorkNote(ctx, existing, note); err != nil {
				e.logger.Warn("work note append failed, will retry", "alert_id", alertID, "error", err)
				return Retry
			}
			e.logger.Info("resolving alert recorded on existing incident", "incident_number", existing.IncidentNumber, "alert_id", alertID)
			return Processed
		}

		// A duplicate alert against a still-open incident is annotated, not re-notified; against a closed incident it falls through to Upsert and folds into the same row.
		if existing.IsOpen() {
			note := model.BuildWorkNote("Duplicate", alertID, alert.MetricName, alert.Source, time.Now())
			if err := e.incidents.AppendWorkNote(ctx, existing, note); err != nil {
				e.logger.Warn("work note append failed, will retry", "alert_id", alertID, "error", err)
				return Retry
			}
			e.logger.Info("duplicate alert recorded on open incident", "incident_number", existing.IncidentNumber, "alert_id", alertID)
			return Processed
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
	e.deliverAndPersist(ctx, inc)
	return Processed
}

// deliverNotifications attempts CSM first, falling back to Chat only while CSM remains unconfirmed; csmConfirmed and chatNotified report each channel's outcome independently, skipping already-satisfied ones.
func (e *Engine) deliverNotifications(ctx context.Context, inc model.Incident) (csmConfirmed, chatNotified bool) {
	csmConfirmed = inc.CSMConfirmed
	if !csmConfirmed {
		if csmNumber, ok := e.notifier.NotifyCSM(ctx, inc); ok {
			if csmNumber != inc.IncidentNumber {
				if err := e.incidents.UpdateIncidentNumber(ctx, inc.Fingerprint, csmNumber); err != nil {
					e.logger.Error("failed to persist csm-assigned incident number", "incident_number", inc.IncidentNumber, "csm_incident_number", csmNumber, "error", err)
				}
			}
			csmConfirmed = true
		}
	}

	chatNotified = inc.Notified
	if !csmConfirmed && !chatNotified {
		// CSM didn't confirm (this attempt or a prior one), so fall back to
		// FALLBACK_CHAT_WEBHOOK_URLS so a human still sees it -- but only the first time; once
		// delivered, resending on every later CSM-retry sweep would just spam the space.
		chatNotified = e.notifier.NotifyChat(ctx, inc)
	}
	return csmConfirmed, chatNotified
}

// deliverAndPersist runs deliverNotifications and persists whichever flag newly flipped to true,
// logging (not failing) any persist error the same way the rest of this file treats notification
// bookkeeping as best-effort, non-blocking side work.
func (e *Engine) deliverAndPersist(ctx context.Context, inc model.Incident) {
	csmConfirmed, chatNotified := e.deliverNotifications(ctx, inc)
	if csmConfirmed && !inc.CSMConfirmed {
		if err := e.incidents.MarkCSMConfirmed(ctx, inc.Fingerprint); err != nil {
			e.logger.Error("failed to persist csm-confirmed flag", "incident_number", inc.IncidentNumber, "error", err)
		}
	}
	if chatNotified && !inc.Notified {
		if err := e.incidents.MarkNotified(ctx, inc.Fingerprint); err != nil {
			e.logger.Error("failed to persist notified flag", "incident_number", inc.IncidentNumber, "error", err)
		}
	}
}

// RetrySweep re-attempts delivery for every incident still owed a CSM confirmation, a Chat notification, or both; it is safe to call concurrently with Process, since each incident row is keyed by fingerprint and the engine never processes the same fingerprint at once.
func (e *Engine) RetrySweep(ctx context.Context) {
	pending, err := e.incidents.ListPending(ctx)
	if err != nil {
		e.logger.Error("notify retry sweep: failed to list pending incidents", "error", err)
		return
	}
	for _, inc := range pending {
		e.deliverAndPersist(ctx, inc)
	}
}
