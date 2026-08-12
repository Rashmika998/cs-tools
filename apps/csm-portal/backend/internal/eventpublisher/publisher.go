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

// Package eventpublisher is the producer side of the domain-event pipeline
// csm-notification-service consumes from: it builds the wire envelope
// {type, entityId, payload} that service's internal/events.Envelope
// expects, publishes it to Event Hub via internal/eventbus, and — if Event
// Hub doesn't acknowledge the publish — durably records that failure via
// entity-service's POST /event-publish-failures instead of losing the event
// silently. csm-notification-service never talks to that table itself
// anymore (it's a pure Kafka consumer); this package is the one writer.
package eventpublisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/events"
)

// recordFailureTimeout bounds the failure-recording call below — it runs on
// a context.WithoutCancel copy of the caller's ctx (see Publish), so without
// its own bound it could hang indefinitely if entity-service is slow or
// unreachable, now that it's no longer tied to the caller's own deadline.
const recordFailureTimeout = 10 * time.Second

// kafkaProducer abstracts eventbus.Producer for testability.
type kafkaProducer interface {
	Publish(ctx context.Context, key, value []byte) error
}

// entityClient abstracts entity.CustomerEntityClient's event-publish-failure
// call for testability.
type entityClient interface {
	CreateEventPublishFailure(ctx context.Context, body []byte) ([]byte, error)
}

// Publisher publishes domain events for csm-notification-service to consume.
type Publisher struct {
	kafka  kafkaProducer
	entity entityClient
}

// New constructs a Publisher.
func New(kafka kafkaProducer, entity entityClient) *Publisher {
	return &Publisher{kafka: kafka, entity: entity}
}

// Publish builds the envelope for eventType/entityID/payload and publishes
// it to Event Hub, keyed by entityID (so every event about the same
// case/incident stays ordered on the same partition — see
// eventbus.Producer.Publish).
//
// If the publish itself fails (Event Hub never acknowledges it), Publish
// makes a best-effort call to entity-service to durably record the failure
// (for manual remediation later — see domain.EventPublishFailure on the
// entity-service side), then still returns the original publish error: from
// the caller's perspective the event was not delivered to the bus, and
// recording that fact is a side effect, not a substitute for delivery. That
// call runs on a context.WithoutCancel copy of ctx (bounded by its own
// recordFailureTimeout, not ctx's deadline) — ctx may already be canceled or
// about to expire by the time a slow Kafka write finally errors out, and a
// canceled context would make the HTTP call to entity-service fail
// instantly, defeating the whole point of recording the failure. If the
// failure-recording call *also* fails, that's logged here (not returned)
// rather than compounding the error the caller already has to handle.
//
// KNOWN GAP: a publish error does not prove Event Hub rejected the record —
// the write can still land while only the acknowledgement is lost (a network
// blip after the broker appended it). Manually republishing from the
// recorded failure in that case would duplicate the event, since neither the
// envelope nor the failure record carries a stable event ID a consumer could
// dedupe on. Closing this needs an event ID threaded through the envelope,
// the failure record, and a durable dedupe check on the consumer side (or
// entity-service) — a real design addition, not a quick fix, so it's flagged
// here rather than built speculatively.
func (p *Publisher) Publish(ctx context.Context, eventType events.Type, entityID string, payload json.RawMessage) error {
	body, err := json.Marshal(events.Envelope{Type: eventType, EntityID: entityID, Payload: payload})
	if err != nil {
		return fmt.Errorf("eventpublisher: encode envelope: %w", err)
	}

	pubErr := p.kafka.Publish(ctx, []byte(entityID), body)
	if pubErr == nil {
		return nil
	}

	failureBody, err := json.Marshal(struct {
		EventType string          `json:"eventType"`
		EntityID  string          `json:"entityId"`
		Payload   json.RawMessage `json:"payload"`
		Error     string          `json:"error"`
	}{EventType: string(eventType), EntityID: entityID, Payload: payload, Error: pubErr.Error()})
	if err != nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and could not encode the failure record", "eventType", eventType, "entityId", entityID, "publishErr", pubErr, "encodeErr", err)
		return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
	}

	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordFailureTimeout)
	defer cancel()
	if _, recErr := p.entity.CreateEventPublishFailure(recordCtx, failureBody); recErr != nil {
		slog.ErrorContext(ctx, "eventpublisher: publish failed and recording the failure also failed", "eventType", eventType, "entityId", entityID, "publishErr", pubErr, "recordErr", recErr)
	}

	return fmt.Errorf("eventpublisher: publish %s for entity %s: %w", eventType, entityID, pubErr)
}
