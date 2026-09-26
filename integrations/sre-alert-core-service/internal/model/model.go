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

// Package model holds alert/incident shapes and severity/fingerprint normalization for the incident pipeline.
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

// Alert is the canonical shape stored by alert-ingestion-service; read and normalized here before dedup.
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

// Incident dedups alerts by fingerprint; Severity is numeric (1=Critical..5=OK); db tags drive gocqlx binding.
type Incident struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint"`
	// IncidentID is CSM's UUID for PATCH; empty until confirmed. IncidentNumber is the human-readable display id.
	IncidentID     string   `json:"incident_id" db:"incident_id"`
	IncidentNumber string   `json:"incident_number" db:"incident_number"`
	Status         string   `json:"status" db:"status"`
	Severity       int      `json:"severity" db:"severity"`
	Impact         string   `json:"impact" db:"impact"`
	Urgency        string   `json:"urgency" db:"urgency"`
	Service        string   `json:"service" db:"service"`
	MetricName     string   `json:"metric_name" db:"metric_name"`
	Description    string   `json:"description" db:"description"`
	Category       string   `json:"category" db:"category"`
	Environment    string   `json:"environment" db:"environment"`
	Source         string   `json:"source" db:"source"`
	AlertIDs       []string `json:"alert_ids" db:"alert_ids"`
	AlertCount     int      `json:"alert_count" db:"alert_count"`
	WorkNotes      []string `json:"work_notes" db:"work_notes"`
	// PendingNotes is the FIFO subset of WorkNotes not yet confirmed pushed to CSM; cleared as CSM accepts
	// each one, in order. Separate from WorkNotes (the full local audit log) because WorkNotes is capped
	// and tail-trimmed, which would misalign a simple "notes pushed so far" counter.
	PendingNotes []string  `json:"pending_notes" db:"pending_notes"`
	FirstSeen    time.Time `json:"first_seen" db:"first_seen"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	// StateCheckedAt throttles how often syncIncidentState calls CSM to refresh Status, so a flapping
	// alert on a confirmed incident doesn't cost one CSM round trip per duplicate during a storm.
	StateCheckedAt time.Time `json:"state_checked_at" db:"state_checked_at"`
	// Notified and CSMConfirmed are independent obligations, each retried separately until true.
	Notified     bool `json:"notified" db:"notified"`
	CSMConfirmed bool `json:"csm_confirmed" db:"csm_confirmed"`
	// CSMAttempts caps retries so permanently-rejected (4xx) payloads stop being rescanned; CSMPermanentlyFailed then excludes the row from ListPending.
	CSMAttempts          int  `json:"csm_attempts" db:"csm_attempts"`
	CSMPermanentlyFailed bool `json:"csm_permanently_failed" db:"csm_permanently_failed"`
}

// IsOpen defaults to true until CSM confirms "closed", since this service never closes incidents and unsynced rows must not look closed.
// A permanently-failed incident is treated as closed too: CSM will never confirm it, so without this
// every later alert on the same fingerprint would be folded into it as a silent local Duplicate note
// forever, with no further CSM attempt and no further Chat message. Upsert's existing closed-incident
// handling already resets delivery state and starts a fresh generation, which is exactly what a
// permanently-failed incident needs on its next occurrence.
func (i Incident) IsOpen() bool {
	if i.CSMPermanentlyFailed {
		return false
	}
	if !i.CSMConfirmed {
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(i.Status), "closed")
}

// Defaults are fallback Alert field values sourced from the CORE_ALERT_DEFAULTS env var.
type Defaults struct {
	Service     string `json:"service"`
	MetricName  string `json:"metric_name"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Environment string `json:"environment"`
	Source      string `json:"source"`
}

// LoadDefaults fails loudly on malformed JSON rather than silently dropping configured fallbacks.
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

// Apply fills empty Alert fields from Defaults; already-populated fields are left untouched.
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

// numericSeverity maps lowercase severity labels to this service's internal 0-5 scale.
var numericSeverity = map[string]int{
	"critical": 1,
	"major":    2,
	"minor":    3,
	"warning":  4,
	"ok":       5,
	"clear":    0,
}

// SeverityToNumeric defaults unrecognized labels to 1 (Critical) as fail-safe; callers should log when recognized is false.
func SeverityToNumeric(label string) (n int, recognized bool) {
	if n, ok := numericSeverity[strings.ToLower(strings.TrimSpace(label))]; ok {
		return n, true
	}
	return 1, false
}

// IsResolving reports whether severityNum is a recovery signal (OK=5 or Clear=0).
func IsResolving(severityNum int) bool {
	return severityNum == 5 || severityNum == 0
}

// ImpactUrgency maps severity to CSM's Impact/Urgency strings ("HIGH"/"MEDIUM"/"LOW"), per CreateIncidentRequest's contract.
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

// BuildWorkNote formats a journal entry, referencing the alert by id rather than an instance URL link.
func BuildWorkNote(kind, alertID, metricName, source string, at time.Time) string {
	metricName = firstNonEmpty(metricName, "N/A")
	source = firstNonEmpty(source, "N/A")
	return fmt.Sprintf("%s\n%s alert received\nAlert ID: %s\nMetric: %s\nSource: %s",
		at.UTC().Format(time.RFC3339), kind, alertID, metricName, source)
}

// Fingerprint is the dedup key; a distinct unique identifier always starts a new incident.
func Fingerprint(source, service, metricName, environment, uniqueIdentifier string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{source, service, metricName, environment, uniqueIdentifier}, "|")))
	return hex.EncodeToString(sum[:])
}
