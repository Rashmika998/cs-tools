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

// Package caseevents is a consumer of the case-events Kafka topic, in its
// own consumer group (see cmd/server/main.go) so it gets its own full copy
// of every event independent of csm-notification-service's consumers.
// Deliberately minimal for now: Handler just logs what it receives. The
// intended real use — pushing case updates (comment added, etc.) to the
// frontend in real time — is a follow-up; this package is the scaffold
// that'll grow into that.
package caseevents

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/events"
)

// Handler reacts to events on the case-events topic. Currently: logs
// type/entityId and nothing else.
type Handler struct{}

// NewHandler constructs a Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Handle implements eventbus.Handle. Never returns an error today — a
// malformed record is logged and dropped rather than retried, since
// nothing here has a failure mode retrying would fix (see
// eventbus.Consumer.Run's doc comment on why this consumer skips
// retry/dead-letter handling for now).
//
// Deliberately does not log payload: it can carry PII (recipient emails,
// comment text, etc. — see the case.* payloads in csm-notification-service's
// internal/events), and this backend's own logging convention is IDs and
// error summaries only, never request/event bodies.
func (h *Handler) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		slog.ErrorContext(ctx, "caseevents: failed to decode event", "err", err)
		return nil
	}
	slog.InfoContext(ctx, "caseevents: received case event", "type", env.Type, "entityId", env.EntityID)
	return nil
}
