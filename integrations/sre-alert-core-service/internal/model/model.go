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

// Package model holds the shared alert and incident shapes, plus the severity and fingerprint normalization rules ported from ServiceNow's AlertCoreUtils (see PLAN.md sections 19-20).
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
	Fingerprint    string    `json:"fingerprint" db:"fingerprint"`
	IncidentNumber string    `json:"incident_number" db:"incident_number"`
	Status         string    `json:"status" db:"status"`
	Severity       int       `json:"severity" db:"severity"`
	Impact         int       `json:"impact" db:"impact"`
	Urgency        int       `json:"urgency" db:"urgency"`
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
	// Notified is true once Chat delivered to every target; CSMConfirmed is true once CSM assigned a real incident_number — independent obligations, each retried until true.
	Notified     bool `json:"notified" db:"notified"`
	CSMConfirmed bool `json:"csm_confirmed" db:"csm_confirmed"`
}

// openStatuses are the incident lifecycle states that ServiceNow's original flow treated as state less than or equal to 2, meaning New or In Progress.
var openStatuses = map[string]bool{"new": true, "in progress": true}

// IsOpen reports whether the incident is still actionable by checking its status against openStatuses, mirroring ServiceNow's original "state is lesser than or is 2" check.
func (i Incident) IsOpen() bool {
	return openStatuses[strings.ToLower(strings.TrimSpace(i.Status))]
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

// numericSeverity maps a lowercase severity label like "critical" or "warning" to ServiceNow's numeric 0-5 severity scale used throughout incident processing.
var numericSeverity = map[string]int{
	"critical": 1,
	"major":    2,
	"minor":    3,
	"warning":  4,
	"ok":       5,
	"clear":    0,
}

// SeverityToNumeric converts a severity label to its numeric value via numericSeverity, defaulting any empty or unrecognized label to 1, meaning Critical.
func SeverityToNumeric(label string) int {
	if n, ok := numericSeverity[strings.ToLower(strings.TrimSpace(label))]; ok {
		return n
	}
	return 1
}

// IsResolving reports whether severityNum equals OK (5) or Clear (0), meaning the alert is a recovery signal rather than a newly reported problem.
func IsResolving(severityNum int) bool {
	return severityNum == 5 || severityNum == 0
}

// ImpactUrgency maps a numeric severity to CSM's Impact/Urgency pair, ported directly from ServiceNow's original Create Record scripted mapping table.
func ImpactUrgency(severityNum int) (impact, urgency int) {
	switch severityNum {
	case 1:
		return 1, 1
	case 2:
		return 2, 1
	case 3:
		return 2, 2
	case 4:
		return 2, 3
	case 5:
		return 3, 3
	default:
		return 1, 1
	}
}

// BuildWorkNote formats a journal entry matching ServiceNow's scripted work notes, referencing the alert by id rather than an instance URL link.
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
