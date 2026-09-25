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

// incidentColumns lists the incidents table's columns once; model.Incident's db tags map them via gocqlx.
var incidentColumns = []string{
	"fingerprint", "incident_number", "status", "severity", "impact", "urgency", "service",
	"metric_name", "description", "category", "environment", "source", "alert_ids", "alert_count", "work_notes",
	"first_seen", "last_seen", "notified", "csm_confirmed",
}

// IncidentRepo owns the incidents table; dedup uniqueness is a lightweight CAS transaction on fingerprint.
type IncidentRepo struct {
	session gocqlx.Session
}

// NewIncidentRepo wraps session for incident storage.
func NewIncidentRepo(session *gocql.Session) (*IncidentRepo, error) {
	return &IncidentRepo{session: gocqlx.NewSession(session)}, nil
}

// pendingIncidentNumber is a placeholder until CSM assigns the real one, derived from fingerprint so it's deterministic.
func pendingIncidentNumber(fingerprint string) string {
	return "PENDING-" + fingerprint[:12]
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
			Status:         "New",
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
		stmt, names := qb.Insert("incidents_proccessed").Columns(incidentColumns...).Unique().ToCql()
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
			ins, insNames := qb.Insert("incidents_proccessed").Columns(incidentColumns...).ToCql()
			if err := r.session.Query(ins, insNames).WithContext(ctx).BindStruct(inc).ExecRelease(); err != nil {
				return model.Incident{}, false, fmt.Errorf("create incident %s (unconditional after stale CAS): %w", fp, err)
			}
			return inc, true, nil
		}
	}

	// Skip if alertID is already folded in, e.g. a retry after an ambiguous but successful timeout.
	for _, seen := range existing.AlertIDs {
		if seen == alertID {
			return existing, false, nil
		}
	}

	updated := existing
	updated.AlertIDs = append(append([]string{}, existing.AlertIDs...), alertID)
	updated.AlertCount = existing.AlertCount + 1
	updated.Severity = min(existing.Severity, severityNum) // lower number = more severe
	updated.LastSeen = time.Now().UTC()
	if updated.Category == "" && a.Category != "" {
		// Self-heal: an incident with no category yet picks one up from a later alert instead of staying blank.
		updated.Category = a.Category
	}
	if updated.Description == "" && a.Description != "" {
		// Self-heal: same as Category, so an incident created before its first descriptive alert still fills in.
		updated.Description = a.Description
	}

	stmt, names := qb.Update("incidents_proccessed").
		Set("alert_ids", "alert_count", "severity", "category", "description", "last_seen").
		Where(qb.Eq("fingerprint")).
		ToCql()
	if err := r.session.Query(stmt, names).WithContext(ctx).BindStruct(updated).ExecRelease(); err != nil {
		return model.Incident{}, false, fmt.Errorf("update incident %s: %w", fp, err)
	}
	return updated, false, nil
}

// UpdateIncidentNumber replaces the placeholder incident_number with the real one CSM assigned, after Upsert has run.
func (r *IncidentRepo) UpdateIncidentNumber(ctx context.Context, fingerprint, incidentNumber string) error {
	stmt, names := qb.Update("incidents_proccessed").
		Set("incident_number").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint":     fingerprint,
		"incident_number": incidentNumber,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("update incident number for %s: %w", fingerprint, err)
	}
	return nil
}

// FindByFingerprint reads the incident matching an alert's fingerprint without mutating anything, so the engine can decide whether to annotate or Upsert before touching any row.
func (r *IncidentRepo) FindByFingerprint(ctx context.Context, fp string) (model.Incident, bool, error) {
	return r.get(ctx, fp)
}

// MarkNotified flips notified to true once Chat has delivered to every configured target.
func (r *IncidentRepo) MarkNotified(ctx context.Context, fingerprint string) error {
	stmt, names := qb.Update("incidents_proccessed").
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

// MarkCSMConfirmed flips csm_confirmed to true once CSM has assigned the real incident_number.
// Independent of MarkNotified: CSM confirming doesn't imply Chat ever delivered, and vice versa.
func (r *IncidentRepo) MarkCSMConfirmed(ctx context.Context, fingerprint string) error {
	stmt, names := qb.Update("incidents_proccessed").
		Set("csm_confirmed").
		Where(qb.Eq("fingerprint")).
		ToCql()
	err := r.session.Query(stmt, names).WithContext(ctx).BindMap(qb.M{
		"fingerprint":   fingerprint,
		"csm_confirmed": true,
	}).ExecRelease()
	if err != nil {
		return fmt.Errorf("mark incident %s csm-confirmed: %w", fingerprint, err)
	}
	return nil
}

func (r *IncidentRepo) ListPending(ctx context.Context) ([]model.Incident, error) {
	stmt, names := qb.Select("incidents_proccessed").Columns(incidentColumns...).ToCql()
	var all []model.Incident
	if err := r.session.Query(stmt, names).WithContext(ctx).SelectRelease(&all); err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	pending := make([]model.Incident, 0, len(all))
	for _, inc := range all {
		// Chat is only owed while CSM is unconfirmed; CSM confirmation closes both obligations.
		if !inc.CSMConfirmed {
			pending = append(pending, inc)
		}
	}
	return pending, nil
}

// AppendWorkNote appends note to existing's work notes and writes the full list back, mirroring Upsert's Go-side-append pattern for alert_ids rather than relying on Cosmos's Cassandra API to support native list append.
func (r *IncidentRepo) AppendWorkNote(ctx context.Context, existing model.Incident, note string) error {
	updated := append(append([]string{}, existing.WorkNotes...), note)
	stmt, names := qb.Update("incidents_proccessed").
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
	stmt, names := qb.Select("incidents_proccessed").Columns(incidentColumns...).Where(qb.Eq("fingerprint")).ToCql()
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
