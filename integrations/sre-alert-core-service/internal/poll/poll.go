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

// Package poll discovers new alerts by comparing alert_seq against alert_cursor; ping wakes it early, a ticker is the backstop.
package poll

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"

	"alert-core-service/internal/cassandra"
	"alert-core-service/internal/engine"
	"alert-core-service/internal/model"
)

const (
	alertSeqTable = "alert_seq"
	cursorTable   = "alert_cursor"
	alertIDPrefix = "ALT"
	alertIDWidth  = 9
)

// Leader reports whether this replica is the elected active processor. Only the leader processes
// alerts, so multiple replicas never send duplicate incident notifications.
type Leader interface {
	IsLeader() bool
}

// Settings tunes a Poller's cadence and per-cycle concurrency.
type Settings struct {
	// Interval is the backstop cadence between cycles; a ping normally wakes the poller sooner.
	Interval time.Duration
	// Concurrency is the number of fingerprint-sharded workers handling alerts in parallel.
	Concurrency int
	// ReadConcurrency bounds the parallel alert-row reads at the start of each cycle.
	ReadConcurrency int
	// MaxWindow caps how many alert ids one window processes, bounding memory under large bursts.
	MaxWindow int
	// NotifySweepInterval is the cadence for retrying incidents whose CSM/Chat notification hasn't yet confirmed; it is independent of the alert processing cycle and is safe to call concurrently with it.
	NotifySweepInterval time.Duration
	// GapTimeout bounds how long a single missing alert id (its row never became visible, e.g. the producer bumped alert_seq and died before writing it) is allowed to block every id after it. Once an id has been the head of every window for longer than this, it is skipped (logged loudly) instead of retried forever. Zero disables the bound, restoring the old block-forever behavior.
	GapTimeout time.Duration
}

// Poller periodically (and on demand) processes every alert id issued since its last confirmed position.
type Poller struct {
	logger   *slog.Logger
	session  *gocql.Session
	engine   *engine.Engine
	leader   Leader
	settings Settings
	wake     chan struct{}
	sweeping atomic.Bool
	// wg tracks in-flight work launched off the Run goroutine (currently only RetrySweep), so Run doesn't return -- and a caller waiting on it doesn't consider the poller drained -- while a sweep is still delivering to CSM/Chat.
	wg sync.WaitGroup
	// stuck backs GapTimeout above. Only ever touched from the single goroutine that calls cycle (Run's own select loop), so it needs no lock.
	stuck stuckTracker
}

// New seeds the alert_seq and cursor rows (idempotent) and returns a ready poller.
// alert_seq must be seeded here too: on a fresh deployment with no ingestion writer
// having run yet, an unseeded alert_seq makes every cycle's first read fail with
// "not found" forever, so the poller never even reaches the alerts table.
func New(logger *slog.Logger, session *gocql.Session, e *engine.Engine, leader Leader, settings Settings) (*Poller, error) {
	if err := cassandra.SeedSeq(context.Background(), session, alertSeqTable); err != nil {
		return nil, err
	}
	if err := cassandra.SeedSeq(context.Background(), session, cursorTable); err != nil {
		return nil, err
	}
	return &Poller{
		logger:   logger,
		session:  session,
		engine:   e,
		leader:   leader,
		settings: settings,
		wake:     make(chan struct{}, 1),
	}, nil
}

// Wake nudges the poller to run now instead of waiting; non-blocking, so a ping burst collapses into one cycle.
func (p *Poller) Wake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Run processes alerts on ping and on a fixed interval backstop until ctx is cancelled; cycles never
// overlap. A second, independent ticker retries any incident whose CSM/Chat notification is still
// outstanding, so a Chat/CSM outage recovers on its own without ever blocking alert ingestion.
//
// Callers that need to know the poller has actually drained before proceeding (e.g. main releasing
// the processor lease on shutdown) should give Run a context of its own, cancel it, and then wait for
// Run to return: it does not return until every RetrySweep goroutine it launched has also finished,
// even though cycle() itself always runs synchronously on this same goroutine.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.settings.Interval)
	defer ticker.Stop()
	notifyTicker := time.NewTicker(p.settings.NotifySweepInterval)
	defer notifyTicker.Stop()
	defer p.wg.Wait()

	p.cycle(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cycle(ctx)
		case <-p.wake:
			p.cycle(ctx)
		case <-notifyTicker.C:
			if p.leader.IsLeader() && p.sweeping.CompareAndSwap(false, true) {
				// Runs off the poll loop's goroutine so a slow CSM/Chat outage during the sweep never delays cycle(); the guard keeps sweeps from overlapping themselves. RetrySweep rechecks leadership per incident and stops if it's lost mid-sweep. Tracked in p.wg so Run doesn't report itself drained while this is still in flight.
				p.wg.Add(1)
				go func() {
					defer p.wg.Done()
					defer p.sweeping.Store(false)
					p.engine.RetrySweep(ctx, p.leader.IsLeader)
				}()
			}
		}
	}
}

// cycle drains alert ids from cursor to the latest issued, one bounded window at a time, advancing
// the durable cursor across each window's completed prefix. Only the elected leader does work.
func (p *Poller) cycle(ctx context.Context) {
	if !p.leader.IsLeader() {
		return
	}

	latest, err := cassandra.ReadSeq(ctx, p.session, alertSeqTable)
	if err != nil {
		p.logger.Error("failed to read alert_seq", "error", err)
		return
	}
	cursor, err := cassandra.ReadSeq(ctx, p.session, cursorTable)
	if err != nil {
		p.logger.Error("failed to read cursor", "error", err)
		return
	}

	for cursor < latest {
		if !p.leader.IsLeader() {
			p.logger.Warn("lost leadership mid-cycle, stopping", "cursor", cursor)
			return
		}
		next := p.processWindow(ctx, cursor, latest)
		if next <= cursor {
			// No forward progress (head of window is not visible yet, or another writer moved the cursor first), so stop this cycle and wait for the next tick or ping.
			return
		}
		cursor = next
	}
}

// processWindow reads, handles, and confirms one window of alert ids in [cursor+1, min(latest, cursor+MaxWindow)] and returns the new cursor position. It never blocks waiting for a future alert id to appear; any unready id is deferred to the next cycle.
func (p *Poller) processWindow(ctx context.Context, cursor, latest int64) int64 {
	base := cursor + 1
	end := min(latest, cursor+int64(p.settings.MaxWindow))
	n := int(end - base + 1)

	// Stage 1: read + normalize every id in the window concurrently, into disjoint slots.
	slots := p.readWindow(ctx, base, n)

	// Determine the ready prefix: handling stops at the first id not yet visible (Retry). An id that is visible but failed to normalize (Failed) is skipped past, so the next cycle re-reads it and retries.
	outcomes := make([]engine.Outcome, n)
	readStop := n
	for i := range n {
		if slots[i].ready {
			continue
		}
		if slots[i].outcome == engine.Retry {
			readStop = i
			break
		}
		outcomes[i] = slots[i].outcome // Failed: terminal, skip past it
	}
	for i := readStop; i < n; i++ {
		outcomes[i] = engine.Retry // deferred to the next cycle
	}

	// A missing row at the very head of the window (readStop == 0) blocks every id after it forever, with no bound -- if the producer bumped alert_seq and died before writing that row, the cursor never advances past it and all alert processing halts silently. GapTimeout caps how long this service waits before skipping the stuck id, logging loudly so the gap is visible rather than silent.
	if n > 0 && p.stuck.observe(base, readStop == 0, time.Now(), p.settings.GapTimeout) {
		id := cassandra.FormatSeq(alertIDPrefix, alertIDWidth, base)
		p.logger.Error("alert id stuck beyond gap timeout, skipping to unblock the pipeline", "alert_id", id, "gap_timeout", p.settings.GapTimeout)
		outcomes[0] = engine.Failed
	}

	p.handleSharded(ctx, base, slots, outcomes, readStop)

	// Advance across the leading run of completed ids (Processed/Failed), stopping at the first Retry.
	completed := contiguousCompleted(outcomes)
	if completed == 0 {
		return cursor
	}
	target := base + int64(completed) - 1

	applied, err := cassandra.AdvanceSeqTo(ctx, p.session, cursorTable, cursor, target)
	if err != nil {
		p.logger.Error("failed to advance cursor", "from", cursor, "to", target, "error", err)
		return cursor
	}
	if !applied {
		p.logger.Warn("cursor advanced concurrently, stopping cycle")
		return cursor
	}
	p.logger.Info("processed alert window", "from", base, "to", target, "count", completed)
	return target
}

// prepared holds the result of reading and normalizing one alert id.
type prepared struct {
	alert   model.Alert
	fp      string
	outcome engine.Outcome
	ready   bool
}

// readWindow reads n alert ids starting at base concurrently, bounded by ReadConcurrency.
func (p *Poller) readWindow(ctx context.Context, base int64, n int) []prepared {
	slots := make([]prepared, n)
	sem := make(chan struct{}, p.settings.ReadConcurrency)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			id := cassandra.FormatSeq(alertIDPrefix, alertIDWidth, base+int64(i))
			alert, fp, outcome, ready := p.engine.Prepare(ctx, id)
			slots[i] = prepared{alert: alert, fp: fp, outcome: outcome, ready: ready}
		}(i)
	}
	wg.Wait()
	return slots
}

// handleSharded handles the ready ids in slots[0:readStop] on a fingerprint-sharded worker pool and writes each outcome into outcomes[0:readStop]. It returns only after all workers finish, so the caller can advance the cursor across the leading run of completed ids.
func (p *Poller) handleSharded(ctx context.Context, base int64, slots []prepared, outcomes []engine.Outcome, readStop int) {
	workers := min(p.settings.Concurrency, readStop)
	if workers <= 0 {
		return // nothing ready to handle
	}

	// Size each worker's queue to exactly its assignment count so dispatch never blocks.
	shardOf := make([]int, readStop)
	counts := make([]int, workers)
	for i := range readStop {
		if !slots[i].ready {
			continue
		}
		s := shard(slots[i].fp, workers)
		shardOf[i] = s
		counts[s]++
	}

	type task struct {
		idx int
		id  string
	}
	queues := make([]chan task, workers)
	var wg sync.WaitGroup
	for w := range workers {
		queues[w] = make(chan task, counts[w])
		wg.Add(1)
		go func(q chan task) {
			defer wg.Done()
			for t := range q {
				outcomes[t.idx] = p.engine.Handle(ctx, t.id, slots[t.idx].alert)
			}
		}(queues[w])
	}
	for i := range readStop {
		if !slots[i].ready {
			continue
		}
		id := cassandra.FormatSeq(alertIDPrefix, alertIDWidth, base+int64(i))
		queues[shardOf[i]] <- task{idx: i, id: id}
	}
	for w := range workers {
		close(queues[w])
	}
	wg.Wait()
}

// contiguousCompleted returns the length of the leading run of completed outcomes (Processed or Failed) in the slice, stopping at the first Retry. The caller uses this to advance the cursor across the window's completed prefix.
func contiguousCompleted(outcomes []engine.Outcome) int {
	for i, o := range outcomes {
		if o == engine.Retry {
			return i
		}
	}
	return len(outcomes)
}

// shard maps a fingerprint to one of workers worker queues via FNV-1a, so a given incident's alerts always land on the same worker.
func shard(fingerprint string, workers int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(fingerprint))
	return int(h.Sum32() % uint32(workers))
}
