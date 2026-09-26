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

package poll

import (
	"testing"
	"time"

	"alert-core-service/internal/engine"
)

func TestContiguousCompleted(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []engine.Outcome
		want     int
	}{
		{"all processed", []engine.Outcome{engine.Processed, engine.Processed, engine.Processed}, 3},
		{"retry stops the run immediately", []engine.Outcome{engine.Retry, engine.Processed}, 0},
		{"gap in the middle stops at the retry", []engine.Outcome{engine.Processed, engine.Retry, engine.Processed}, 1},
		{"failed counts as completed", []engine.Outcome{engine.Processed, engine.Failed, engine.Processed}, 3},
		{"empty", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := contiguousCompleted(tc.outcomes); got != tc.want {
				t.Errorf("contiguousCompleted(%v) = %d, want %d", tc.outcomes, got, tc.want)
			}
		})
	}
}

func TestShardIsStableForSameFingerprint(t *testing.T) {
	const workers = 8
	fp := "some-fingerprint"
	first := shard(fp, workers)
	for range 100 {
		if got := shard(fp, workers); got != first {
			t.Fatalf("shard(%q, %d) = %d, want stable %d", fp, workers, got, first)
		}
	}
	if first < 0 || first >= workers {
		t.Fatalf("shard(%q, %d) = %d, out of range", fp, workers, first)
	}
}

func TestStuckTrackerSkipsOnlyAfterGapTimeout(t *testing.T) {
	var s stuckTracker
	now := time.Unix(1_700_000_000, 0)
	gap := 5 * time.Minute

	// First time this id is the head of the window: record it, don't skip yet.
	if skip := s.observe(42, true, now, gap); skip {
		t.Fatalf("observe: expected no skip on first sighting")
	}

	// Same id, still blocked, but well under the gap timeout.
	if skip := s.observe(42, true, now.Add(time.Minute), gap); skip {
		t.Fatalf("observe: expected no skip before gap timeout elapses")
	}

	// Same id, blocked for longer than the gap timeout: skip it exactly once.
	if skip := s.observe(42, true, now.Add(gap+time.Second), gap); !skip {
		t.Fatalf("observe: expected skip once gap timeout elapses")
	}

	// Having just skipped, state resets -- the same id blocked again starts a fresh window.
	if skip := s.observe(42, true, now.Add(gap+2*time.Second), gap); skip {
		t.Fatalf("observe: expected no skip immediately after a reset")
	}
}

func TestStuckTrackerNeverSkipsWhenGapTimeoutDisabled(t *testing.T) {
	var s stuckTracker
	now := time.Unix(1_700_000_000, 0)
	for i := range 10 {
		if skip := s.observe(7, true, now.Add(time.Duration(i)*time.Hour), 0); skip {
			t.Fatalf("observe: expected never to skip when gapTimeout <= 0")
		}
	}
}

func TestStuckTrackerResetsOnProgress(t *testing.T) {
	var s stuckTracker
	now := time.Unix(1_700_000_000, 0)
	gap := time.Minute

	s.observe(1, true, now, gap)
	if s.at != 1 {
		t.Fatalf("expected tracker to record id 1 as stuck, got %d", s.at)
	}

	// id 1 becomes ready (still the window's head, but no longer blocked), so the tracker
	// must forget it rather than counting time against a since-resolved id if it ever gets
	// stuck again later.
	s.observe(1, false, now.Add(30*time.Second), gap)
	if s.at != 0 {
		t.Fatalf("expected tracker to clear after progress, got at=%d", s.at)
	}
}
