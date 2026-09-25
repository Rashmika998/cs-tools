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

// Package notify posts incidents to CSM as the primary channel, assigning incident_number, and falls back to Chat webhooks with retry and exponential backoff.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"

	"alert-core-service/internal/model"
)

// Notifier posts prepared incidents to downstream systems over HTTP, targeting CSM first and falling back to Google Chat webhooks when CSM does not confirm.
type Notifier struct {
	logger                  *slog.Logger
	client                  *http.Client
	csmWebhookURL           string
	fallbackChatWebhookURLs []string
	maxAttempts             int
	retryBaseDelay          time.Duration
}

// New builds a notifier from CSM_WEBHOOK_URL and FALLBACK_CHAT_WEBHOOK_URLS; maxAttempts, retryBaseDelay, and httpTimeout tune per-target retry behavior and are all deployment-configurable via config.toml.
func New(logger *slog.Logger, maxAttempts int, retryBaseDelay, httpTimeout time.Duration) *Notifier {
	n := &Notifier{
		logger:                  logger,
		client:                  &http.Client{Timeout: httpTimeout},
		csmWebhookURL:           os.Getenv("CSM_WEBHOOK_URL"),
		fallbackChatWebhookURLs: splitURLs(os.Getenv("FALLBACK_CHAT_WEBHOOK_URLS")),
		maxAttempts:             maxAttempts,
		retryBaseDelay:          retryBaseDelay,
	}
	if n.csmWebhookURL == "" {
		logger.Warn("CSM_WEBHOOK_URL not set; incidents will not be forwarded to CSM")
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

// csmResponse is CSM's JSON reply; its incident_number replaces the deterministic placeholder value that IncidentRepo.Upsert originally seeded when the incident row was created.
type csmResponse struct {
	IncidentNumber string `json:"incident_number"`
}

// NotifyCSM posts inc to the CSM webhook and returns its assigned incident_number; ok is false on any failure, which the caller uses to trigger Chat fallback.
func (n *Notifier) NotifyCSM(ctx context.Context, inc model.Incident) (incidentNumber string, ok bool) {
	if n.csmWebhookURL == "" {
		return "", false
	}
	body, err := n.postWithRetry(ctx, n.csmWebhookURL, inc)
	if err != nil {
		n.logger.Error("notify failed after retries", "target", "csm", "incident_number", inc.IncidentNumber, "error", err)
		return "", false
	}
	var resp csmResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.IncidentNumber == "" {
		n.logger.Error("csm response missing incident_number", "incident_number", inc.IncidentNumber, "error", err)
		return "", false
	}
	n.logger.Info("notified", "target", "csm", "incident_number", resp.IncidentNumber)
	return resp.IncidentNumber, true
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
	for _, url := range n.fallbackChatWebhookURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			spaceID := chatSpaceID(url)
			if _, err := n.postWithRetry(ctx, url, card); err != nil {
				n.logger.Error("notify failed after retries", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber, "error", err)
				failures.Add(1)
				return
			}
			n.logger.Info("notified", "target", "google_chat", "chat_space_id", spaceID, "incident_number", inc.IncidentNumber)
		}(url)
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

// postWithRetry retries transient failures (network, 5xx, 429) with backoff; other 4xx responses fail immediately, and 429 stays retryable since webhook targets rate-limit bursty concurrent incidents.
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

// post makes exactly one HTTP attempt against url, returning the response status code (0 if the request never got a response) and the body on success.
func (n *Notifier) post(ctx context.Context, url string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
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

// priorityLabel formats a numeric severity as CSM's expected priority string convention, "P<N> - <Word>", for example rendering severity 1 as "P1 - Critical".
func priorityLabel(severity int) string {
	return fmt.Sprintf("P%d - %s", severity, severityLabel(severity))
}

// fallbackGoogleChatCard builds a cardsV2 message mirroring CSM's incident card layout, titled with a FALLBACK prefix since this path has no team-specific routing information.
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
