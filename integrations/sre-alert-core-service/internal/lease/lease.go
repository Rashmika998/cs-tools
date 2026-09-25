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

// Package lease elects a single active alert processor across replicas via a time-bounded LWT lease, so Choreo can run multiple alert-core-service containers without duplicate incidents.
package lease

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"
)

// leaseTable and leaseKey are fixed because exactly one processor lease row exists for the whole deployment; no per-tenant or per-shard leases are needed here.
const (
	leaseTable = "processor_lease"
	leaseKey   = "poller"
)

// epoch is written to expires_at to mark the lease row free and unheld, both when it is first seeded and whenever a leader releases it on shutdown.
var epoch = time.Unix(0, 0).UTC()

// Lease is a single-holder, time-bounded lock; exactly one replica's Lease reports IsLeader at a time, and on holder death the lease expires after ttl and a standby steals it.
type Lease struct {
	session *gocql.Session
	logger  *slog.Logger
	owner   string
	ttl     time.Duration
	leader  atomic.Bool
}

// Identity returns a per-process owner id built from the hostname plus a random suffix, so concurrently running replicas each get a value unique to their process.
func Identity() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "pod"
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return host + "-" + hex.EncodeToString(b[:])
}

// New seeds the free lease row if it is absent and returns a ready Lease that starts out as a non-leader until its first successful Run tick.
func New(session *gocql.Session, logger *slog.Logger, owner string, ttl time.Duration) (*Lease, error) {
	l := &Lease{session: session, logger: logger, owner: owner, ttl: ttl}
	if err := l.seed(context.Background()); err != nil {
		return nil, err
	}
	return l, nil
}

// seed inserts a free lease row, with owner empty and expiry already elapsed, so the first replica to attempt acquisition can steal it via a normal CAS write.
func (l *Lease) seed(ctx context.Context) error {
	_, err := l.session.Query(
		fmt.Sprintf(`INSERT INTO %s (name, owner, expires_at) VALUES (?, '', ?) IF NOT EXISTS`, leaseTable),
		leaseKey, epoch,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return fmt.Errorf("seed lease: %w", err)
	}
	return nil
}

// IsLeader reports whether this replica currently holds the lease, based on the result of the most recent Run tick rather than a fresh Cassandra read.
func (l *Lease) IsLeader() bool {
	return l.leader.Load()
}

// Run acquires or renews the lease every renewInterval until ctx is cancelled, updating IsLeader; renewInterval must stay well under ttl so one missed renewal never drops it.
func (l *Lease) Run(ctx context.Context, renewInterval time.Duration) {
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	l.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.tick(ctx)
		}
	}
}

// tick runs one acquire-or-renew attempt against the lease row and logs whenever this replica's leadership status flips between leader and standby.
func (l *Lease) tick(ctx context.Context) {
	wasLeader := l.leader.Load()
	isLeader, err := l.tryAcquireOrRenew(ctx)
	if err != nil {
		// Can't confirm leadership, so stand down; the row still names us until expiry, so no standby steals meanwhile — a transient blip costs seconds, not correctness.
		l.leader.Store(false)
		l.logger.Warn("lease acquire/renew failed, standing down", "error", err)
		return
	}
	switch {
	case isLeader && !wasLeader:
		l.logger.Info("acquired lease, now active processor", "owner", l.owner)
	case !isLeader && wasLeader:
		l.logger.Info("lost lease, now standby", "owner", l.owner)
	}
}

// tryAcquireOrRenew reads the current lease row and, depending on its owner and expiry, either renews our own hold, steals an expired one, or backs off as standby.
func (l *Lease) tryAcquireOrRenew(ctx context.Context) (bool, error) {
	var owner string
	var expiresAt time.Time
	if err := l.session.Query(
		fmt.Sprintf(`SELECT owner, expires_at FROM %s WHERE name = ?`, leaseTable), leaseKey,
	).WithContext(ctx).Scan(&owner, &expiresAt); err != nil {
		return false, fmt.Errorf("read lease: %w", err)
	}

	now := time.Now().UTC()
	newExpiry := now.Add(l.ttl)

	switch {
	case owner == l.owner:
		// We already hold it, so renew by guarding the CAS on our own ownership; a concurrent steal attempt still wins cleanly if it happens first.
		return l.cas(ctx, l.owner, newExpiry)
	case expiresAt.After(now):
		// A live lease is held by someone else, so this replica stays standby until it expires or this replica observes it freed.
		l.leader.Store(false)
		return false, nil
	default:
		// Expired or freed, so steal it, guarding on the exact owner value observed; Cosmos LWT only supports equality, so time is compared here and CAS enforces the race.
		return l.cas(ctx, owner, newExpiry)
	}
}

// cas sets this replica as owner with newExpiry only if the row's current owner still equals expectedOwner, so a concurrent writer's CAS wins instead of silently overwriting it.
func (l *Lease) cas(ctx context.Context, expectedOwner string, newExpiry time.Time) (bool, error) {
	applied, err := l.session.Query(
		fmt.Sprintf(`UPDATE %s SET owner = ?, expires_at = ? WHERE name = ? IF owner = ?`, leaseTable),
		l.owner, newExpiry, leaseKey, expectedOwner,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return false, fmt.Errorf("cas lease: %w", err)
	}
	l.leader.Store(applied)
	return applied, nil
}

// Release frees the lease if this replica still holds it, so a standby can take over immediately instead of waiting out the ttl; it is best-effort and meant for graceful shutdown.
func (l *Lease) Release(ctx context.Context) error {
	l.leader.Store(false)
	_, err := l.session.Query(
		fmt.Sprintf(`UPDATE %s SET owner = '', expires_at = ? WHERE name = ? IF owner = ?`, leaseTable),
		epoch, leaseKey, l.owner,
	).WithContext(ctx).MapScanCAS(map[string]any{})
	if err != nil {
		return fmt.Errorf("release lease: %w", err)
	}
	return nil
}
