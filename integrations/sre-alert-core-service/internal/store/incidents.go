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

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
	"github.com/scylladb/gocqlx/v2/qb"

	"alert-core-service/internal/model"
)

var incidentColumns = []string{
	"fingerprint", "incident_id", "incident_number", "status", "severity", "impact", "urgency", "service",
	"metric_name", "description", "category", "environment", "source", "alert_ids", "alert_count", "work_notes",
	"first_seen", "last_seen", "notified", "csm_confirmed", "csm_attempts", "csm_permanently_failed",
}

// Bounds unbounded lists so a flapping alert can't blow past Cosmos's row-size limit; AlertCount keeps growing regardless.
const (
	maxAlertIDs  = 500
	maxWorkNotes = 200
)

// IncidentRepo owns the incidents table; dedup uniqueness is a lightweight CAS transaction on fingerprint.
type IncidentRepo struct {
	session gocqlx.Session
}

func NewIncidentRepo(session *gocql.Session) (*IncidentRepo, error) {
	return &IncidentRepo{session: gocqlx.NewSession(session)}, nil
}

// pendingIncidentNumber is a placeholder until CSM assigns the real one, derived from fingerprint so it's deterministic.
func pendingIncidentNumber(fingerprint string) string {
	return "PENDING-" + fingerprint[:12]
}

// capTail keeps only the last max entries of list, so a long-lived, flapping incident's stored history stays bounded.
func capTail[T any](list []T, max int) []T {
	if len(list) <= max {
		return list
	}
	return append([]T{}, list[len(list)-max:]...)
}

// Upsert maps the alert onto an incident by fingerprint, creating it via IF NOT EXISTS or updating it.
func (r *IncidentRepo) Upsert(ctx context.Context, alertID string, a model.Alert, severityNum int) (model.Incident, bool, error) {
	fp := model.Fingerprint(a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier)

	existing, found, err := r.get(ctx, fp)
	if err != nil {
		return model.Incident{}, false, err
	}

	if !found {
		now := time.Now().UTC()
		impact, urgency := model.ImpactUrgency(severityNum)
		inc := model.Incident{
			Fingerprint:    fp,
			IncidentNumber: pendingIncidentNumber(fp),
			Status:         "new",
			Severity:       severityNum,
			Impact:         impact,
			Urgency:        urgency,
			Service:        a.Service,
			MetricName:     a.MetricName,
			Description:    a.Description,
			Category:       a.Category,
			Environment:    a.Environment,
			Source:         a.Source,
			AlertIDs:       []string{alertID},
			AlertCount:     1,
			FirstSeen:      now,
			LastSeen:       now,
		}
		stmt, names := qb.Insert("incidents_processed").Columns(incidentColumns...).Unique().ToCql()
		applied, err := r.session.Query(stmt, names).WithContext(ctx).BindStruct(inc).ExecCASRelease()
		if err != nil {
			return model.Incident{}, false, fmt.Errorf("create incident %s: %w", fp, err)
		}
		if applied {
			return inc, true, nil
		}
		// Lost the race to another core; fall through and treat this alert as an update.
		existing, found, err = r.get(ctx, fp)
		if err != nil {
			return model.Incident{}, false, err
		}
		if !found {
			ins, insNames := qb.Insert("incidents_processed").Columns(incidentColumns...).ToCql()
			if err := r.session.Query(ins, insNames).WithContext(ctx).BindStruct(inc).ExecRelease(); err != nil {
				return model.Incident{}, false, fmt.Errorf("create incident %s (unconditional after stale CAS): %w", fp, err)
			}
			return inc, true, nil
		}
	}

	// Skip if alertID already folded in (retry after ambiguous timeout); engine.Handle's idempotency check relies on this too.
	for _, seen := range existing.AlertIDs {
		if seen == alertID {
			return existing, false, nil
		}
	}

	updated := existing
	updated.AlertIDs = capTail(append(append([]string{}, existing.AlertIDs...), alertID), maxAlertIDs)
	updated.AlertCount = existing.AlertCount + 1
	if severityNum < existing.Severity { // lower number = more severe
		updated.Severity = severityNum
		// Impact/Urgency must be recomputed on escalation -- otherwise a Minor->Critical incident keeps its original, now-stale pair.
		updated.Impact, updated.Urgency = model.ImpactUrgency(severityNum)
	}
	updated.LastSeen = time.Now().UTC()
	if updated.Category == "" && a.Category != "" {
		// Self-heal: an incident with no category yet picks one up from a later alert instead of staying blank.
		updated.Category = a.Category
	}
	if updated.Description == "" && a.Description != "" {
		// Self-heal: same as Category, so an incident created before its first descriptive alert still fills in.
		updated.Description = a.Description
	}

	setCols := []string{"alert_ids", "alert_count", "severity", "impact", "urgency", "category", "description", "last_seen"}
	// Handle only reaches here for a closed incident: reset delivery fields or the recurrence is
	// silently swallowed. FirstSeen reset also gives DedupTag a fresh value for NotifyCSM.
	if !existing.IsOpen() {
		updated.Status = "new"
		updated.IncidentID = ""
		updated.IncidentNumber = pendingIncidentNumber(fp)
		updated.Notified = false
		updated.CSMConfirmed = false
		updated.CSMAttempts = 0
		updated.CSMPermanentlyFailed = false
		updated.FirstSeen = updated.LastSeen
		setCols = append(setCols, "status", "incident_id", "incident_number", "notified", "csm_confirmed", "csm_attempts", "csm_permanently_failed", "first_seen")
	}

	stmt, names := qb.Update("incidents_processed").
		Set(setCols...).
		Where(qb.Eq("fingerprint")).
		ToCql()
	if err := r.session.Query(stmt, names).WithContext(ctx).BindStruct(updated).ExecRelease(); err != nil {
		return model.Incident{}, false, fmt.Errorf("update incident %s: %w", fp, err)
	}
	return updated, false, nil
}

// RecordAlertID appends alertID for idempotent annotate-only paths, matching Upsert's dedup check; no-op if already present.
func (r *IncidentRepo) RecordAlertID(ctx context.Context, existing model.Incident, alertID string) error {
	for _, seen := range existing.AlertIDs {
		if seen == alertID {
			return nil
		}
	}
	updated := capTail(append(append([]string{}, existing.AlertIDs...), alertID), maxAlertIDs)
	stmt, names := qb.Update("incidents_processed").
		Set("alert_ids").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint": existing.Fingerprint,
		"alert_ids":   updated,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("record alert id on incident %s: %w", existing.Fingerprint, err)
	}
	return nil
}

// RecordCSMIncident writes id, number, and csm_confirmed together so confirmed is never observed with a placeholder id.
func (r *IncidentRepo) RecordCSMIncident(ctx context.Context, fingerprint, incidentID, incidentNumber string) error {
	stmt, names := qb.Update("incidents_processed").
		Set("incident_id", "incident_number", "csm_confirmed").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint":     fingerprint,
		"incident_id":     incidentID,
		"incident_number": incidentNumber,
		"csm_confirmed":   true,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("record csm incident for %s: %w", fingerprint, err)
	}
	return nil
}

// RecordCSMAttemptFailure sets csm_permanently_failed once attempts are exhausted or CSM rejects non-retryably, stopping RetrySweep from retrying forever.
func (r *IncidentRepo) RecordCSMAttemptFailure(ctx context.Context, fingerprint string, attempts, maxAttempts int, permanent bool) error {
	failed := permanent || attempts >= maxAttempts
	stmt, names := qb.Update("incidents_processed").
		Set("csm_attempts", "csm_permanently_failed").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint":            fingerprint,
		"csm_attempts":           attempts,
		"csm_permanently_failed": failed,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("record csm attempt failure for %s: %w", fingerprint, err)
	}
	return nil
}

// SyncStatus persists CSM's status so IsOpen reflects CSM's lifecycle, not a value only this service wrote.
func (r *IncidentRepo) SyncStatus(ctx context.Context, fingerprint, status string) error {
	stmt, names := qb.Update("incidents_processed").
		Set("status").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint": fingerprint,
		"status":      status,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("sync status for %s: %w", fingerprint, err)
	}
	return nil
}

// FindByFingerprint reads without mutating, so the engine can decide annotate vs Upsert before touching any row.
func (r *IncidentRepo) FindByFingerprint(ctx context.Context, fp string) (model.Incident, bool, error) {
	return r.get(ctx, fp)
}

// MarkNotified flips notified to true once Chat has delivered to every configured target.
func (r *IncidentRepo) MarkNotified(ctx context.Context, fingerprint string) error {
	stmt, names := qb.Update("incidents_processed").
		Set("notified").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint": fingerprint,
		"notified":    true,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("mark incident %s notified: %w", fingerprint, err)
	}
	return nil
}

func (r *IncidentRepo) ListPending(ctx context.Context) ([]model.Incident, error) {
	stmt, names := qb.Select("incidents_processed").Columns(incidentColumns...).ToCql()
	var all []model.Incident
	if err := r.session.Query(stmt, names).WithContext(ctx).SelectRelease(&all); err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	pending := make([]model.Incident, 0, len(all))
	for _, inc := range all {
		// Chat is owed only while CSM is unconfirmed; permanently-rejected rows are excluded to avoid retrying forever.
		if !inc.CSMConfirmed && !inc.CSMPermanentlyFailed {
			pending = append(pending, inc)
		}
	}
	return pending, nil
}

// AppendWorkNote appends and rewrites the full list, since Cosmos's Cassandra API lacks native list append.
func (r *IncidentRepo) AppendWorkNote(ctx context.Context, existing model.Incident, note string) error {
	updated := capTail(append(append([]string{}, existing.WorkNotes...), note), maxWorkNotes)
	stmt, names := qb.Update("incidents_processed").
		Set("work_notes").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint": existing.Fingerprint,
		"work_notes":  updated,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("append work note to incident %s: %w", existing.Fingerprint, err)
	}
	return nil
}

// get reads the incident for a fingerprint, reporting absence as false rather than an error.
func (r *IncidentRepo) get(ctx context.Context, fp string) (model.Incident, bool, error) {
	stmt, names := qb.Select("incidents_processed").Columns(incidentColumns...).Where(qb.Eq("fingerprint")).ToCql()
	var inc model.Incident
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{"fingerprint": fp}).GetRelease(&inc)
	if err == gocql.ErrNotFound {
		return model.Incident{}, false, nil
	}
	if err != nil {
		return model.Incident{}, false, fmt.Errorf("read incident %s: %w", fp, err)
	}
	return inc, true, nil
}
