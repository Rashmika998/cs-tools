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

package slaengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// entityClock abstracts EntityClient for testability.
type entityClock interface {
	RegisterClock(ctx context.Context, req RegisterClockRequest) error
	GetClock(ctx context.Context, caseID, clockType string) (Clock, error)
	SetTierReachedIfUnset(ctx context.Context, caseID, clockType, tier string) (reachedAt time.Time, alreadyReached bool, err error)
}

// wakeIndex abstracts WakeIndex for testability.
type wakeIndex interface {
	AddWake(ctx context.Context, member string, at time.Time) error
	RemoveWake(ctx context.Context, member string) error
	DueMembers(ctx context.Context, now time.Time) ([]string, error)
}

// eventPublisher abstracts eventbus.Producer for testability.
type eventPublisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// chatSender abstracts notifications.GoogleChatClient's SendSLABreachAlert
// for testability.
type chatSender interface {
	SendSLABreachAlert(ctx context.Context, product, clockType, tier, caseNumber, wso2CaseID, caseTitle, caseType, productName, team, severity, state, openedAt, caseLink string) error
}

// linkResolver abstracts recipientlinks.Resolver's CSMLink for testability
// — the only method this engine needs from it (unlike
// internal/dispatch's own, larger linkResolver interface).
type linkResolver interface {
	CSMLink(caseID string) string
}

// tiers is the fixed set of elapsed-duration percentages this engine tracks
// per clock — 50%, 75%, 100% of the way from startedAt to dueAt. Not
// configurable: a real per-severity policy (see events.SLAClockRegisterPayload's
// doc comment) could one day vary durations, but the three checkpoints
// within a clock are a fixed part of this engine's own design, ported as-is
// from the POC.
var tiers = []string{"50", "75", "100"}

// Engine is the SLA timer engine: Handle registers clocks (the consumer
// side, wired into its own eventbus.Consumer — see cmd/server/main.go),
// RunTicker/Tick scan the wake index, publish tier-crossing events, and —
// on a genuine breach — send a Google Chat alert directly (see
// sendBreachAlert; not routed through internal/dispatch).
type Engine struct {
	entity entityClock
	wake   wakeIndex
	pub    eventPublisher
	chat   chatSender
	links  linkResolver
	// defaultChatProduct is sendBreachAlert's fallback when a clock's own
	// stored Product (from registration) is empty — same
	// "publisher didn't say" fallback reasoning as
	// dispatch.Dispatcher.defaultChatProduct, reusing the same configured
	// DEFAULT_CHAT_PRODUCT value (see cmd/server/main.go).
	defaultChatProduct string
}

// NewEngine constructs an Engine.
func NewEngine(entity *EntityClient, wake *WakeIndex, pub *eventbus.Producer, chat *notifications.GoogleChatClient, links *recipientlinks.Resolver, defaultChatProduct string) *Engine {
	return &Engine{entity: entity, wake: wake, pub: pub, chat: chat, links: links, defaultChatProduct: defaultChatProduct}
}

// Handle implements eventbus.Handle for the SLA engine's own consumer group.
// It shares a topic with events unrelated to this engine (case.*,
// incident.created, and its own sla.tier_reached output) — anything other
// than events.TypeSLAClockRegister is silently ignored, not an error,
// mirroring dispatch.Dispatcher.Handle's own no-op case for these two new
// types (see dispatch.go).
func (e *Engine) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("slaengine: decode envelope: %w", err)
	}
	if env.Type != events.TypeSLAClockRegister {
		return nil
	}
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("slaengine: invalid payload: %w", err)
	}

	var p events.SLAClockRegisterPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("slaengine: decode sla.clock.register payload: %w", err)
	}
	return e.registerClocks(ctx, p)
}

// registerClocks (re)creates every named clock and its 50/75/100% wake
// entries — used for both first registration and a re-registration that
// wipes and rebuilds a clock from scratch (entity-service's RegisterClock
// endpoint always resets on conflict — see its own doc comment). By the time
// Handle calls this, events.Validate has already rejected the whole record
// if any duration in it failed to parse (mirroring this package's own
// validRecipients: one bad entry fails the whole event, not just that one
// clock) — the time.ParseDuration re-check below is defensive belt-and-
// suspenders, not a reachable path through Handle's own call chain.
func (e *Engine) registerClocks(ctx context.Context, p events.SLAClockRegisterPayload) error {
	var errs []error
	for clockType, durStr := range p.Durations {
		d, err := time.ParseDuration(durStr)
		if err != nil {
			slog.ErrorContext(ctx, "slaengine: invalid sla clock duration, skipping", "caseId", p.CaseID, "clockType", clockType, "duration", durStr, "err", err)
			continue
		}

		startedAt := time.Now()
		dueAt := startedAt.Add(d)
		if slices.Contains(p.AvoidWeekendDueDate, clockType) {
			dueAt = avoidWeekend(dueAt)
		}
		if err := e.entity.RegisterClock(ctx, RegisterClockRequest{
			CaseID: p.CaseID, ClockType: clockType, StartedAt: startedAt, DueAt: dueAt,
			CaseNumber: p.CaseNumber, WSO2CaseID: p.WSO2CaseID, CaseTitle: p.CaseTitle,
			CaseType: p.CaseType, Product: p.Product, Team: p.Team, Priority: p.Priority,
		}); err != nil {
			errs = append(errs, fmt.Errorf("register clock %s/%s: %w", p.CaseID, clockType, err))
			continue
		}

		for _, tier := range tiers {
			at := tierTime(startedAt, d, tier)
			member := wakeMember(p.CaseID, clockType, tier)
			if err := e.wake.AddWake(ctx, member, at); err != nil {
				errs = append(errs, fmt.Errorf("add wake entry %s: %w", member, err))
			}
		}
		slog.InfoContext(ctx, "slaengine: registered sla clock", "caseId", p.CaseID, "clockType", clockType, "dueAt", dueAt.Format(time.RFC3339))
	}
	return errors.Join(errs...)
}

// avoidWeekend rolls due forward to the next Monday, same time-of-day, if
// it falls on a Saturday or Sunday — see events.SLAClockRegisterPayload's
// own AvoidWeekendDueDate doc comment for what this approximates ("1
// Business Week") and why it's applied here rather than in entity-service.
func avoidWeekend(due time.Time) time.Time {
	switch due.Weekday() {
	case time.Saturday:
		return due.AddDate(0, 0, 2)
	case time.Sunday:
		return due.AddDate(0, 0, 1)
	default:
		return due
	}
}

// tierTime returns the instant tier is reached, given a clock that started
// at startedAt and runs for duration d.
func tierTime(startedAt time.Time, d time.Duration, tier string) time.Time {
	switch tier {
	case "50":
		return startedAt.Add(d / 2)
	case "75":
		return startedAt.Add(d * 3 / 4)
	default: // "100"
		return startedAt.Add(d)
	}
}

// Tick scans the wake index for members due at or before now, and for each:
// checks the clock isn't paused, records the tier reached (idempotently),
// publishes events.TypeSLATierReached, and only then removes the wake
// entry — in that order, so a publish failure leaves the wake entry in
// place and retries on the next tick instead of silently losing it (the
// entity-service write already happened and is itself idempotent, so
// re-attempting it on a retry is harmless).
//
// ACCEPTED TRADE-OFF: processDueMember gates publishing on
// SetTierReachedIfUnset's alreadyReached (skip if some earlier call already
// claimed the tier) — a deliberate choice, made with eyes open to what it
// costs. alreadyReached only reports whether the database claim succeeded,
// not whether a notification was ever actually delivered: if the caller
// that won the claim then fails to publish (Event Hub rejects the publish,
// or this process crashes between the database write and the publish
// call), a later rediscovery of that same tier will see alreadyReached=true
// and skip publishing — the notification is then permanently lost, not
// just delayed. That risk is judged acceptable here because a publish
// failure to Azure Event Hub is rare, and clean, duplicate-free rediscovery
// matters routinely, not just on the rare occasion a replica races
// another: a planned (not yet built) fallback for when Redis itself is
// unreachable — falling back to asking entity-service directly which
// tiers are overdue — would, once Redis recovers, rediscover whatever
// stale wake entries survived the outage. Without this gating, every one
// of those would duplicate-publish on every recovery, not just
// occasionally; that routine cost is what actually motivated keeping this
// gating rather than the rare multi-replica race alone.
//
// If this trade-off ever stops being acceptable (e.g. Event Hub reliability
// turns out worse in practice, or this service starts running multiple
// replicas), the real fix is a durable delivery/outbox state tracked
// separately from the reached-claim, with a lease or expiry so a failed
// attempt's slot can still be retried by someone else — not built, and a
// real addition, not a quick one.
func (e *Engine) Tick(ctx context.Context, now time.Time) error {
	members, err := e.wake.DueMembers(ctx, now)
	if err != nil {
		return fmt.Errorf("slaengine: scan due members: %w", err)
	}

	var errs []error
	for _, member := range members {
		if err := e.processDueMember(ctx, member); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// processDueMember handles one due wake-index member: checks the clock
// isn't paused, claims the tier as reached on entity-service, and skips
// publishing (just drops the wake entry) if that claim reports
// alreadyReached — see Tick's own doc comment for the trade-off this
// accepts. Otherwise it publishes events.TypeSLATierReached and only then
// removes the wake entry, so a publish failure on this specific call
// leaves the entry in place for the next tick to retry.
func (e *Engine) processDueMember(ctx context.Context, member string) error {
	caseID, clockType, tier, ok := parseWakeMember(member)
	if !ok {
		slog.ErrorContext(ctx, "slaengine: malformed wake member, dropping", "member", member)
		return e.wake.RemoveWake(ctx, member)
	}

	clock, err := e.entity.GetClock(ctx, caseID, clockType)
	if err != nil {
		return fmt.Errorf("get clock %s/%s: %w", caseID, clockType, err)
	}
	if clock.PausedOn != nil {
		slog.InfoContext(ctx, "slaengine: clock is paused, dropping wake entry", "caseId", caseID, "clockType", clockType)
		return e.wake.RemoveWake(ctx, member)
	}

	reachedAt, alreadyReached, err := e.entity.SetTierReachedIfUnset(ctx, caseID, clockType, tier)
	if err != nil {
		return fmt.Errorf("set tier reached %s/%s/%s: %w", caseID, clockType, tier, err)
	}
	if alreadyReached {
		// A deliberate, accepted trade-off — see this function's own doc
		// comment above for the full reasoning: this skips publishing on
		// the rare chance that the caller who won the claim already
		// published but this caller can't tell that from "claimed but
		// never got to publish." Chosen because a duplicate-free rediscovery
		// path matters routinely (the planned Redis-outage fallback would
		// otherwise duplicate on every recovery), while the residual risk
		// this accepts — a publish to Event Hub failing, or this process
		// crashing between the database write and the publish call — is
		// judged rare enough to live with.
		slog.InfoContext(ctx, "slaengine: tier already reached by another caller, dropping stale wake entry without republishing",
			"caseId", caseID, "clockType", clockType, "tier", tier, "reachedAt", reachedAt.Format(time.RFC3339))
		return e.wake.RemoveWake(ctx, member)
	}

	envelope := events.Envelope{
		Type:     events.TypeSLATierReached,
		EntityID: caseID,
	}
	payload, err := json.Marshal(events.SLATierReachedPayload{CaseID: caseID, ClockType: clockType, Tier: tier})
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached payload: %w", err)
	}
	envelope.Payload = payload
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode sla.tier_reached envelope: %w", err)
	}

	if err := e.pub.Publish(ctx, []byte(caseID), body); err != nil {
		return fmt.Errorf("publish sla.tier_reached %s/%s/%s: %w", caseID, clockType, tier, err)
	}

	if err := e.sendBreachAlert(ctx, clock, caseID, clockType, tier); err != nil {
		// Same ordering/retry reasoning as the Kafka publish above: a failed
		// send here leaves the wake entry in place, so the next tick's own
		// SetTierReachedIfUnset call will see alreadyReached=true and skip
		// re-publishing sla.tier_reached (already durably recorded above)
		// but still retry the Chat send — the same accepted
		// possible-duplicate-of-the-already-succeeded-step trade-off this
		// package's Tick doc comment already documents for the Kafka publish
		// itself, just now also covering this second, independent effect.
		return fmt.Errorf("send sla breach alert %s/%s/%s: %w", caseID, clockType, tier, err)
	}

	if err := e.wake.RemoveWake(ctx, member); err != nil {
		return fmt.Errorf("remove wake entry %s after successful publish (at reachedAt %s): %w", member, reachedAt.Format(time.RFC3339), err)
	}
	slog.InfoContext(ctx, "slaengine: sla tier reached", "caseId", caseID, "clockType", clockType, "tier", tier)
	return nil
}

// sendBreachAlert builds and sends the Google Chat breach card for one
// tier crossing, using clock's own display fields (populated once at
// registration — see events.SLAClockRegisterPayload's doc comment) so no
// second lookup is needed here. product falls back to
// e.defaultChatProduct when the clock's own stored Product is empty, same
// reasoning as dispatch.Dispatcher's own Product fallback.
func (e *Engine) sendBreachAlert(ctx context.Context, clock Clock, caseID, clockType, tier string) error {
	product := clock.Product
	if product == "" {
		product = e.defaultChatProduct
	}
	if product == "" {
		slog.WarnContext(ctx, "slaengine: sla breach alert not sent, no Google Chat product configured", "caseId", caseID, "clockType", clockType)
		return nil
	}
	caseNumber := clock.CaseNumber
	if caseNumber == "" {
		// clock.CaseNumber can be empty for a clock registered before this
		// display-data feature existed, or if the publisher omitted it —
		// fall back to the raw case id so the card still has something to
		// show rather than a blank "Case ID :" line.
		caseNumber = caseID
	}
	var openedAt string
	if !clock.StartedOn.IsZero() {
		openedAt = clock.StartedOn.UTC().Format("2006-01-02 15:04:05") + " (UTC)"
	}
	return e.chat.SendSLABreachAlert(ctx, product, clockType, tier, caseNumber, clock.WSO2CaseID, clock.CaseTitle, clock.CaseType, clock.Product, clock.Team, clock.Priority, clock.State, openedAt, e.links.CSMLink(caseID))
}

// RunTicker calls Tick every interval until ctx is done. Run from its own
// goroutine (see cmd/server/main.go); a failed Tick is logged, not fatal —
// the next tick gets another chance at whatever was due.
func (e *Engine) RunTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Tick(ctx, time.Now()); err != nil {
				slog.ErrorContext(ctx, "slaengine: tick failed", "err", err)
			}
		}
	}
}

func wakeMember(caseID, clockType, tier string) string {
	return caseID + "|" + clockType + "|" + tier
}

func parseWakeMember(member string) (caseID, clockType, tier string, ok bool) {
	parts := strings.Split(member, "|")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
