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

// Package notify creates and updates incidents on CSM via csm-integration-service (the platform's real, M2M-authenticated incident API), and falls back to Chat webhooks with retry and exponential backoff when CSM does not confirm.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"

	"alert-core-service/internal/apierror"
	"alert-core-service/internal/csm"
	"alert-core-service/internal/model"
)

// Notifier posts prepared incidents to downstream systems, targeting CSM first via csm-integration-service and falling back to Google Chat webhooks when CSM does not confirm.
type Notifier struct {
	logger                  *slog.Logger
	client                  *http.Client
	csm                     *csm.Client
	callerID                string
	unknownServiceID        string
	services                *serviceCache
	fallbackChatWebhookURLs []string
	maxAttempts             int
	retryBaseDelay          time.Duration
}

// Config is everything New needs beyond the csm itself; kept as a struct rather than a growing positional-argument list.
type Config struct {
	// CallerID is the CSM caller uuid this service authenticates every CreateIncidentRequest as.
	CallerID string
	// UnknownServiceID is the CMDB service uuid used when an alert's raw Service label has no match via SearchServiceID.
	UnknownServiceID string
	// ServiceCacheTTL bounds how long a resolved label->serviceId mapping is reused before a fresh live search.
	ServiceCacheTTL time.Duration
	MaxAttempts     int
	RetryBaseDelay  time.Duration
	HTTPTimeout     time.Duration
}

// New builds a notifier around csm, plus FALLBACK_CHAT_WEBHOOK_URLS for the Chat fallback path.
func New(logger *slog.Logger, csm *csm.Client, cfg Config) *Notifier {
	n := &Notifier{
		logger:                  logger,
		client:                  &http.Client{Timeout: cfg.HTTPTimeout},
		csm:                     csm,
		callerID:                cfg.CallerID,
		unknownServiceID:        cfg.UnknownServiceID,
		services:                newServiceCache(cfg.ServiceCacheTTL),
		fallbackChatWebhookURLs: splitURLs(os.Getenv("FALLBACK_CHAT_WEBHOOK_URLS")),
		maxAttempts:             cfg.MaxAttempts,
		retryBaseDelay:          cfg.RetryBaseDelay,
	}
	if len(n.fallbackChatWebhookURLs) == 0 {
		logger.Warn("FALLBACK_CHAT_WEBHOOK_URLS not set; incidents will not reach Chat if CSM fails")
	}
	return n
}

func splitURLs(raw string) []string {
	var urls []string
	for u := range strings.SplitSeq(raw, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

// DedupTag returns the exact, stable tag embedded in a new incident's
// Subject, keyed off the alert's own fingerprint. NotifyCSM searches for this
// tag before ever calling CreateIncident, so a retried delivery whose
// previous attempt actually succeeded on CSM's side (but whose response was
// lost to this service) reuses the already-created incident instead of
// creating a second one.
func DedupTag(fingerprint string) string {
	return "[fp:" + fingerprint[:12] + "]"
}

// NotifyCSM creates inc as a real CSM incident via csm-integration-service's POST /incidents, first checking for an already-created incident carrying this fingerprint's dedup tag. ok is false on any failure; permanent is true when CSM rejected the payload itself (a 4xx other than 429) and retrying the identical payload can never succeed, so the caller should stop retrying this incident rather than rescanning it on every sweep.
func (n *Notifier) NotifyCSM(ctx context.Context, inc model.Incident) (incidentID, incidentNumber string, ok bool, permanent bool) {
	tag := DedupTag(inc.Fingerprint)
	if id, number, found, err := n.csm.SearchIncidentByTag(ctx, tag); err != nil {
		// Fail open: a search error proves nothing about whether an incident exists, so fall through to attempting CreateIncident as normal, the same posture sre-alert-ingestion-service's own dedup search takes.
		n.logger.Warn("csm dedup search failed, proceeding to create", "incident_number", inc.IncidentNumber, "error", err)
	} else if found {
		n.logger.Info("found existing csm incident via dedup search, reusing", "incident_id", id, "incident_number", number)
		return id, number, true, false
	}

	serviceID, err := n.resolveServiceID(ctx, inc.Service)
	if err != nil {
		n.logger.Error("service id resolution failed, will retry", "incident_number", inc.IncidentNumber, "service", inc.Service, "error", err)
		return "", "", false, false
	}

	req := csm.CreateIncidentRequest{
		CallerID:  n.callerID,
		Category:  csmCategory(inc.Category),
		ServiceID: serviceID,
		Impact:    inc.Impact,
		Urgency:   inc.Urgency,
		Subject:   tag + " " + incidentSubject(inc),
	}

	res, err := n.createIncidentWithRetry(ctx, req)
	if err != nil {
		var apiErr *apierror.Error
		perm := errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusTooManyRequests
		n.logger.Error("csm create incident failed", "incident_number", inc.IncidentNumber, "permanent", perm, "error", err)
		return "", "", false, perm
	}

	n.logger.Info("notified", "target", "csm", "incident_id", res.IncidentID, "incident_number", res.IncidentNumber)
	return res.IncidentID, res.IncidentNumber, true, false
}

// PushWorkNote pushes note onto an already-created CSM incident via PATCH /incidents/{id}, keyed by CSM's own incident id. Best-effort: callers must not fail the overall Outcome on an error here, matching how this service treats every other CSM-side annotation.
func (n *Notifier) PushWorkNote(ctx context.Context, incidentID, note string) error {
	if incidentID == "" {
		return fmt.Errorf("notify: cannot push work note, incident has no csm incident id yet")
	}
	return n.csm.UpdateIncident(ctx, incidentID, note)
}

// IncidentState looks up incidentNumber's current open/closed state on CSM. found is false when CSM has no matching incident (e.g. it hasn't been created yet).
func (n *Notifier) IncidentState(ctx context.Context, incidentNumber string) (open bool, found bool, err error) {
	return n.csm.IncidentState(ctx, incidentNumber)
}

// createIncidentWithRetry retries a transient CreateIncident failure (network error, 5xx, 429) with backoff; a 4xx CSM rejection other than 429 fails immediately, since retrying the same payload can never succeed.
func (n *Notifier) createIncidentWithRetry(ctx context.Context, req csm.CreateIncidentRequest) (*csm.CreateIncidentResult, error) {
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = n.retryBaseDelay
	b := backoff.WithContext(backoff.WithMaxRetries(eb, uint64(n.maxAttempts-1)), ctx)

	var result *csm.CreateIncidentResult
	err := backoff.Retry(func() error {
		res, err := n.csm.CreateIncident(ctx, req)
		if err == nil {
			result = res
			return nil
		}
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != http.StatusTooManyRequests {
			return backoff.Permanent(err)
		}
		return err
	}, b)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveServiceID maps an alert's raw, human-readable Service label to a CMDB service uuid: an in-memory TTL-bounded cache first, then a live POST /services/search, falling back to UnknownServiceID only on a confirmed zero-result search. A transient search error is returned as-is (never translated into the unknown-service fallback), so the caller retries rather than silently mis-tagging the incident.
func (n *Notifier) resolveServiceID(ctx context.Context, label string) (string, error) {
	if label == "" {
		return n.unknownServiceID, nil
	}
	if id, ok := n.services.get(label, time.Now()); ok {
		return id, nil
	}
	id, err := n.csm.SearchServiceID(ctx, label)
	if err != nil {
		return "", err
	}
	if id == "" {
		return n.unknownServiceID, nil
	}
	n.services.set(label, id, time.Now())
	return id, nil
}

// csmCategoryMap translates this service's own free-text alert Category into one of CSM's fixed CreateIncidentRequest.Category enum values.
var csmCategoryMap = map[string]string{
	"security": "SECURITY",
	"inquiry":  "INQUIRY",
}

// csmCategory maps a lowercase category label to CSM's Category enum, defaulting to SERVICE_INTERRUPTION -- the correct default for an alerting pipeline, where the overwhelming majority of incidents represent something breaking rather than a question or a security event.
func csmCategory(category string) string {
	if v, ok := csmCategoryMap[strings.ToLower(strings.TrimSpace(category))]; ok {
		return v
	}
	return "SERVICE_INTERRUPTION"
}

// incidentSubject builds CSM's required one-line Subject from the incident's own fields.
func incidentSubject(inc model.Incident) string {
	subject := inc.Service
	if inc.MetricName != "" {
		subject += " - " + inc.MetricName
	}
	if inc.Environment != "" {
		subject += " (" + inc.Environment + ")"
	}
	return subject
}

// NotifyChat posts inc to every FALLBACK_CHAT_WEBHOOK_URLS target concurrently; ok is true only once all confirm delivery, or immediately if no targets are configured.
func (n *Notifier) NotifyChat(ctx context.Context, inc model.Incident) (ok bool) {
	if len(n.fallbackChatWebhookURLs) == 0 {
		n.logger.Warn("no chat target for incident: FALLBACK_CHAT_WEBHOOK_URLS not configured", "incident_number", inc.IncidentNumber)
		return true
	}
	card := fallbackGoogleChatCard(inc)
	var wg sync.WaitGroup
	var failures atomic.Int32
	for _, chatURL := range n.fallbackChatWebhookURLs {
		wg.Add(1)
		go func(chatURL string) {
			defer wg.Done()
			spaceID := chatSpaceID(chatURL)
			if _, err := n.postWithRetry(ctx, chatURL, card); err != nil {
				n.logger.Error("notify failed after retries", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber, "error", err)
				failures.Add(1)
				return
			}
			n.logger.Info("notified", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber)
		}(chatURL)
	}
	wg.Wait()
	return failures.Load() == 0
}

// chatSpaceID extracts the space id segment from a webhook URL so log lines can identify the target without ever printing the URL's embedded bearer credential.
func chatSpaceID(webhookURL string) string {
	_, rest, ok := strings.Cut(webhookURL, "/spaces/")
	if !ok {
		return "unknown"
	}
	if id, _, ok := strings.Cut(rest, "/"); ok {
		return id
	}
	return "unknown"
}

// postWithRetry retries transient failures (network, 5xx, 429) with backoff; other 4xx responses fail immediately, and 429 stays retryable since Chat webhook targets rate-limit bursty concurrent incidents.
func (n *Notifier) postWithRetry(ctx context.Context, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = n.retryBaseDelay
	b := backoff.WithContext(backoff.WithMaxRetries(eb, uint64(n.maxAttempts-1)), ctx)

	var respBody []byte
	err = backoff.Retry(func() error {
		status, rb, err := n.post(ctx, url, body)
		if err == nil {
			respBody = rb
			return nil
		}
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return backoff.Permanent(err)
		}
		return err
	}, b)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

// post makes exactly one HTTP attempt against target, returning the response status code (0 if the request never got a response) and the body on success.
func (n *Notifier) post(ctx context.Context, target string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			// url.Error.Error() embeds the full request URL, including the webhook's key/token query params; strip it before this error reaches the logs.
			return 0, nil, fmt.Errorf("%s request failed: %w", uerr.Op, uerr.Err)
		}
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return resp.StatusCode, nil, fmt.Errorf("status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// severityWord mirrors model.SeverityToNumeric's numeric severity scale in reverse, title-cased so it can be dropped directly into the Google Chat card header text.
var severityWord = map[int]string{
	1: "Critical",
	2: "Major",
	3: "Minor",
	4: "Warning",
	5: "OK",
	0: "Clear",
}

// severityLabel resolves a numeric severity to its display word (Critical, Major, ...), falling back to the literal string "SEVERITY <N>" outside the known 1-5 scale.
func severityLabel(severity int) string {
	if word, ok := severityWord[severity]; ok {
		return word
	}
	return fmt.Sprintf("SEVERITY %d", severity)
}

// priorityLabel formats a numeric severity as a Chat-card-friendly priority string, for example rendering severity 1 as "P1 - Critical".
func priorityLabel(severity int) string {
	return fmt.Sprintf("P%d - %s", severity, severityLabel(severity))
}

// fallbackGoogleChatCard builds a cardsV2 message summarizing the incident, titled with a FALLBACK prefix since this path has no team-specific routing information.
func fallbackGoogleChatCard(inc model.Incident) map[string]any {
	word := severityLabel(inc.Severity)
	subtitle := "#" + inc.IncidentNumber + " | " + inc.Service
	if inc.Environment != "" {
		subtitle += " | " + inc.Environment
	}
	shortDescription := inc.MetricName
	if shortDescription == "" {
		shortDescription = "No description provided."
	}
	category := inc.Category
	if category == "" {
		category = "Uncategorized"
	}
	return map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": inc.IncidentNumber,
				"card": map[string]any{
					"header": map[string]any{
						"title":    "<font color='#f70707'><b>FALLBACK | " + word + " Priority Incident Reported</b></font>",
						"subtitle": subtitle,
					},
					"sections": []map[string]any{
						{
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": "<b>Short Description:</b><br>" + shortDescription}},
							},
						},
						{
							"header":                    "Incident Details",
							"collapsible":               true,
							"uncollapsibleWidgetsCount": 0,
							"widgets": []map[string]any{
								{"textParagraph": map[string]any{"text": "<b>Category:</b> " + category + "<br>" +
									"<b>Priority:</b> " + priorityLabel(inc.Severity) + "<br>" +
									"<b>State:</b> " + inc.Status}},
							},
						},
					},
				},
			},
		},
	}
}
