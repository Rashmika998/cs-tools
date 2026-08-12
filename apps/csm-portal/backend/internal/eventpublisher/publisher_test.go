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

package eventpublisher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/events"
)

type fakeKafkaProducer struct {
	err              error
	callCount        int
	gotKey, gotValue []byte
}

func (f *fakeKafkaProducer) Publish(ctx context.Context, key, value []byte) error {
	f.callCount++
	f.gotKey, f.gotValue = key, value
	return f.err
}

type entityCall struct {
	body        []byte
	ctxWasDone  bool
	hadDeadline bool
}

type fakeEntityClient struct {
	err   error
	calls []entityCall
}

func (f *fakeEntityClient) CreateEventPublishFailure(ctx context.Context, body []byte) ([]byte, error) {
	_, hasDeadline := ctx.Deadline()
	f.calls = append(f.calls, entityCall{body: body, ctxWasDone: ctx.Err() != nil, hadDeadline: hasDeadline})
	return nil, f.err
}

func TestPublish_Success_DoesNotRecordFailure(t *testing.T) {
	kafka := &fakeKafkaProducer{}
	entity := &fakeEntityClient{}
	p := New(kafka, entity)

	if err := p.Publish(t.Context(), "case.created", "CASE-1", json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
	if kafka.callCount != 1 {
		t.Fatalf("kafka.Publish called %d times, want 1", kafka.callCount)
	}
	if string(kafka.gotKey) != "CASE-1" {
		t.Errorf("publish key = %q, want %q", kafka.gotKey, "CASE-1")
	}

	var env events.Envelope
	if err := json.Unmarshal(kafka.gotValue, &env); err != nil {
		t.Fatalf("published value is not valid JSON: %v", err)
	}
	if env.Type != "case.created" || env.EntityID != "CASE-1" || string(env.Payload) != `{"a":1}` {
		t.Errorf("published envelope = %+v, want type=case.created entityId=CASE-1 payload={\"a\":1}", env)
	}
	if len(entity.calls) != 0 {
		t.Errorf("expected no CreateEventPublishFailure calls on success, got %d", len(entity.calls))
	}
}

func TestPublish_KafkaFails_RecordsFailureAndReturnsOriginalError(t *testing.T) {
	publishErr := errors.New("event hub unreachable")
	kafka := &fakeKafkaProducer{err: publishErr}
	entity := &fakeEntityClient{}
	p := New(kafka, entity)

	err := p.Publish(t.Context(), "case.created", "CASE-1", json.RawMessage(`{"a":1}`))
	if !errors.Is(err, publishErr) {
		t.Fatalf("Publish() err = %v, want it to wrap %v", err, publishErr)
	}
	if len(entity.calls) != 1 {
		t.Fatalf("expected 1 CreateEventPublishFailure call, got %d", len(entity.calls))
	}

	var failure struct {
		EventType string          `json:"eventType"`
		EntityID  string          `json:"entityId"`
		Payload   json.RawMessage `json:"payload"`
		Error     string          `json:"error"`
	}
	if err := json.Unmarshal(entity.calls[0].body, &failure); err != nil {
		t.Fatalf("failure record is not valid JSON: %v", err)
	}
	if failure.EventType != "case.created" || failure.EntityID != "CASE-1" || string(failure.Payload) != `{"a":1}` || failure.Error != publishErr.Error() {
		t.Errorf("failure record = %+v, want eventType=case.created entityId=CASE-1 payload={\"a\":1} error=%q", failure, publishErr.Error())
	}
}

func TestPublish_CanceledContext_StillRecordsFailure(t *testing.T) {
	publishErr := errors.New("event hub unreachable")
	kafka := &fakeKafkaProducer{err: publishErr}
	entity := &fakeEntityClient{}
	p := New(kafka, entity)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // simulate ctx already canceled/expired by the time Publish errors out

	err := p.Publish(ctx, "case.created", "CASE-1", json.RawMessage(`{"a":1}`))
	if !errors.Is(err, publishErr) {
		t.Fatalf("Publish() err = %v, want it to wrap %v", err, publishErr)
	}
	if len(entity.calls) != 1 {
		t.Fatalf("expected 1 CreateEventPublishFailure call even with a canceled ctx, got %d", len(entity.calls))
	}
	if entity.calls[0].ctxWasDone {
		t.Error("CreateEventPublishFailure received a canceled context — it should run on a context.WithoutCancel copy")
	}
	if !entity.calls[0].hadDeadline {
		t.Error("CreateEventPublishFailure's context has no deadline — it should be bounded by recordFailureTimeout")
	}
}

func TestPublish_KafkaFailsAndRecordingAlsoFails_StillReturnsOriginalError(t *testing.T) {
	publishErr := errors.New("event hub unreachable")
	kafka := &fakeKafkaProducer{err: publishErr}
	entity := &fakeEntityClient{err: errors.New("entity-service unreachable")}
	p := New(kafka, entity)

	err := p.Publish(t.Context(), "case.created", "CASE-1", json.RawMessage(`{"a":1}`))
	if !errors.Is(err, publishErr) {
		t.Fatalf("Publish() err = %v, want it to wrap %v even though recording the failure also failed", err, publishErr)
	}
}
