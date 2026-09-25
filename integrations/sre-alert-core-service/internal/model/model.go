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

// Package model holds the shared alert and incident shapes, plus the severity and fingerprint normalization rules this service's incident pipeline is built on.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Alert is the canonical alert shape that the separate alert-ingestion service stores in Cassandra, and that this service reads and normalizes before deduplication.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	Description      string `json:"description"`
}

// Incident dedups alerts by fingerprint; Severity is numeric (1=Critical through 5=OK), and the db struct tags drive gocqlx's column binding when reading and writing rows.
type Incident struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint"`
	// IncidentID is CSM's own incident id (a UUID), the key PATCH /incidents/{id} requires; empty until CSM confirms. IncidentNumber is the human-readable number ("INC0012345") used for display and for searching CSM's open/closed state.
	IncidentID     string    `json:"incident_id" db:"incident_id"`
	IncidentNumber string    `json:"incident_number" db:"incident_number"`
	Status         string    `json:"status" db:"status"`
	Severity       int       `json:"severity" db:"severity"`
	Impact         string    `json:"impact" db:"impact"`
	Urgency        string    `json:"urgency" db:"urgency"`
	Service        string    `json:"service" db:"service"`
	MetricName     string    `json:"metric_name" db:"metric_name"`
	Description    string    `json:"description" db:"description"`
	Category       string    `json:"category" db:"category"`
	Environment    string    `json:"environment" db:"environment"`
	Source         string    `json:"source" db:"source"`
	AlertIDs       []string  `json:"alert_ids" db:"alert_ids"`
	AlertCount     int       `json:"alert_count" db:"alert_count"`
	WorkNotes      []string  `json:"work_notes" db:"work_notes"`
	FirstSeen      time.Time `json:"first_seen" db:"first_seen"`
	LastSeen       time.Time `json:"last_seen" db:"last_seen"`
	// Notified is true once Chat delivered to every target; CSMConfirmed is true once CSM assigned a real IncidentID/IncidentNumber — independent obligations, each retried until true.
	Notified     bool `json:"notified" db:"notified"`
	CSMConfirmed bool `json:"csm_confirmed" db:"csm_confirmed"`
	// CSMAttempts counts failed CreateIncident/PATCH delivery attempts against this incident, so a payload CSM permanently rejects (4xx) stops being retried forever instead of being rescanned by every RetrySweep tick. CSMPermanentlyFailed is set once that cap is hit or CSM reports a non-retryable client error; such rows are excluded from ListPending.
	CSMAttempts          int  `json:"csm_attempts" db:"csm_attempts"`
	CSMPermanentlyFailed bool `json:"csm_permanently_failed" db:"csm_permanently_failed"`
}

// IsOpen reports whether the incident is still actionable. Status here is a
// local snapshot last synced from CSM (see engine.Engine's CSM state sync,
// backed by notify.Notifier.IncidentState) — this service is never the party
// that closes an incident, so Status must never be trusted as authoritative
// on its own between syncs: a row that has never been confirmed by CSM yet
// (no real IncidentNumber) is always still open, since nothing exists on the
// CSM side yet to have closed. This defaults open and requires an explicit
// "closed" to flip, rather than requiring an explicit "open": an incident
// this service just created and confirmed, but hasn't yet had the chance to
// live-sync a state for, must not be mistaken for closed (which would fold a
// later duplicate alert as a *new* incident update on the same row instead
// of annotating the still-open one) just because a sync hasn't happened yet.
func (i Incident) IsOpen() bool {
	if !i.CSMConfirmed {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(i.Status), "closed")
}

// Defaults are the fallback values applied to any Alert field left empty by the upstream source, sourced from the CORE_ALERT_DEFAULTS deployment env var.
type Defaults struct {
	Service     string `json:"service"`
	MetricName  string `json:"metric_name"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Environment string `json:"environment"`
	Source      string `json:"source"`
}

// LoadDefaults reads Defaults from the CORE_ALERT_DEFAULTS env var as JSON; malformed JSON fails loudly rather than silently dropping the configured fallback values.
func LoadDefaults() (Defaults, error) {
	raw := os.Getenv("CORE_ALERT_DEFAULTS")
	if strings.TrimSpace(raw) == "" {
		return Defaults{}, nil
	}
	var d Defaults
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Defaults{}, fmt.Errorf("invalid CORE_ALERT_DEFAULTS: %w", err)
	}
	return d, nil
}

// Apply fills every empty field on an Alert with the corresponding Defaults value, leaving any already-populated field on the alert untouched.
func (d Defaults) Apply(a *Alert) {
	a.Service = firstNonEmpty(a.Service, d.Service)
	a.MetricName = firstNonEmpty(a.MetricName, d.MetricName)
	a.Severity = firstNonEmpty(a.Severity, d.Severity)
	a.Category = firstNonEmpty(a.Category, d.Category)
	a.Environment = firstNonEmpty(a.Environment, d.Environment)
	a.Source = firstNonEmpty(a.Source, d.Source)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// numericSeverity maps a lowercase severity label like "critical" or "warning" to this service's internal numeric 0-5 severity scale used throughout incident processing.
var numericSeverity = map[string]int{
	"critical": 1,
	"major":    2,
	"minor":    3,
	"warning":  4,
	"ok":       5,
	"clear":    0,
}

// SeverityToNumeric converts a severity label to its numeric value via numericSeverity. recognized is false for an empty or unrecognized label; callers should log that case (an unrecognized label silently becoming P1 Critical would otherwise page people for a typo) before falling back to the returned value, which defaults to 1 (Critical) as the safe-by-default choice.
func SeverityToNumeric(label string) (n int, recognized bool) {
	if n, ok := numericSeverity[strings.ToLower(strings.TrimSpace(label))]; ok {
		return n, true
	}
	return 1, false
}

// IsResolving reports whether severityNum equals OK (5) or Clear (0), meaning the alert is a recovery signal rather than a newly reported problem.
func IsResolving(severityNum int) bool {
	return severityNum == 5 || severityNum == 0
}

// ImpactUrgency maps a numeric severity to CSM's Impact/Urgency pair. CSM's
// CreateIncidentRequest (csm-integration-service's openapi.yaml) requires
// these as the strings "HIGH"/"MEDIUM"/"LOW", not integers.
func ImpactUrgency(severityNum int) (impact, urgency string) {
	switch severityNum {
	case 1:
		return "HIGH", "HIGH"
	case 2:
		return "MEDIUM", "HIGH"
	case 3:
		return "MEDIUM", "MEDIUM"
	case 4:
		return "MEDIUM", "LOW"
	case 5:
		return "LOW", "LOW"
	default:
		return "HIGH", "HIGH"
	}
}

// BuildWorkNote formats a journal entry recording an alert event against an incident, referencing the alert by id rather than an instance URL link.
func BuildWorkNote(kind, alertID, metricName, source string, at time.Time) string {
	metricName = firstNonEmpty(metricName, "N/A")
	source = firstNonEmpty(source, "N/A")
	return fmt.Sprintf("%s\n%s alert received\nAlert ID: %s\nMetric: %s\nSource: %s",
		at.UTC().Format(time.RFC3339), kind, alertID, metricName, source)
}

// Fingerprint is the dedup key: a hex SHA-256 hash of source, service, metric, environment, and unique identifier, so a distinct identifier always starts a new incident.
func Fingerprint(source, service, metricName, environment, uniqueIdentifier string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{source, service, metricName, environment, uniqueIdentifier}, "|")))
	return hex.EncodeToString(sum[:])
}
