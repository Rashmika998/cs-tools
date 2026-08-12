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

package eventbus

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"

	kafka "github.com/segmentio/kafka-go"
)

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives — deliberately not the underlying Kafka client's own message
// type, so callers never need to import github.com/segmentio/kafka-go
// directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
}

// Handle processes a single record. Unlike csm-notification-service's own
// eventbus.Handle, a non-nil return here does not trigger a retry — see
// Consumer.Run's doc comment for why.
type Handle func(context.Context, Record) error

// Consumer reads records from a topic as a member of a named consumer
// group, so multiple running instances of a caller split the topic's
// partitions between them instead of each seeing every record. A distinct
// groupID gets its own full copy of every record on the topic, independent
// of every other group — that's the mechanism for adding a second,
// independent reaction to the same events (see internal/caseevents.Handler,
// this package's first caller) without touching the producer or anything
// already consuming the topic.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer constructs a Consumer that joins groupID and consumes
// cfg.Topic. Auto-commit is not used: offsets are committed explicitly by
// Run, only after a record has been handled — never before — so a crash
// mid-processing redelivers the record on restart instead of silently
// skipping it.
func NewConsumer(cfg Config, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Only applies to a partition with no committed offset yet (this
			// consumer group's first run) — read from the beginning, not the
			// tail, matching csm-notification-service's own Consumer.
			StartOffset: kafka.FirstOffset,
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
		}),
	}
}

// Run polls for records and calls handle for each one, committing its
// offset once handle returns — regardless of outcome. Run blocks until ctx
// is canceled or the Consumer is closed; call it from its own goroutine.
//
// Deliberately simpler than csm-notification-service's own Consumer: no
// retry-then-dead-letter policy, since handle's only implementation so far
// (internal/caseevents.Handler) just logs and can't meaningfully fail in a
// way a retry would fix. A handle error is logged here and the record is
// committed anyway — revisit this (retries, a dead-letter topic) once a
// handle exists whose failure modes are actually worth retrying.
func (c *Consumer) Run(ctx context.Context, handle Handle) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
			continue
		}

		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
		}
		if err := handle(ctx, record); err != nil {
			slog.ErrorContext(ctx, "eventbus: handler failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
		}

		if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		}
	}
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
