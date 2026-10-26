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

package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendCaseCreatedAlert_SendsExpectedCard verifies the case.created card:
// no header (a header is always drawn above everything, so the team could
// never lead), then two text widgets with a divider between them. Above it:
// team (bold, in the one team gray), severity (colored) and product (bold).
// Below it: the case reference as a link — with no "new" marker — then the
// case subject as escaped plain text; no button and no separate "View case" line,
// since the case reference itself is the link to the case.
func TestSendCaseCreatedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001001", "WSO2-1000", "WSO2 API Manager",
		`Tom & Jerry <script>`, "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil — a header would sit above the team line", card.Header)
	}
	top, ref, label := cardParts(t, card)
	wantTop := `<font color="#5F6368"><b>Team Nova</b></font><br><font color="#DC2626"><b>Critical (P1)</b></font><br><b>WSO2 API Manager</b>`
	wantRef := `<a href="https://csm.example.com/cases/CASE-1">CS0001001 · WSO2-1000</a>`
	wantLabel := `Tom & Jerry <script>` // a plain label, so verbatim, not HTML-escaped
	if top != wantTop {
		t.Errorf("top text = %q, want %q", top, wantTop)
	}
	if ref != wantRef {
		t.Errorf("case reference text = %q, want %q", ref, wantRef)
	}
	if label != wantLabel {
		t.Errorf("description label = %q, want %q", label, wantLabel)
	}
}

// TestSendCaseCreatedAlert_OmitsEmptyOptionalParts verifies severity/
// product/team are each dropped entirely (no stray " · " separators) when
// the caller doesn't supply one — the linked case reference is never omitted.
func TestSendCaseCreatedAlert_OmitsEmptyOptionalParts(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"", "", "CS0001001", "", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	top, ref, label := cardParts(t, card)
	if top != "" {
		t.Errorf("top text = %q, want none (no team, severity or product)", top)
	}
	if want := `<a href="https://csm.example.com/cases/CASE-1">CS0001001</a>`; ref != want {
		t.Errorf("case reference text = %q, want %q", ref, want)
	}
	if label != "" {
		t.Errorf("description label = %q, want none when there is no title", label)
	}
}

func TestSendCaseCreatedAlert_RejectsEmptyCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseCreatedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendSecurityReportAnalysisAlert_SendsExpectedCard verifies the
// security_report_analysis card's shape: the same headerless two-block layout
// as SendCaseCreatedAlert, but a fixed "Security Report Analysis" label in
// place of a severity line, since severity is never set for this case type.
func TestSendSecurityReportAnalysisAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager",
		"CS0001001", "WSO2-1000", "WSO2 API Manager",
		`Tom & Jerry <script>`, "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSecurityReportAnalysisAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil — a header would sit above the team line", card.Header)
	}
	top, ref, label := cardParts(t, card)
	wantTop := `<font color="#5F6368"><b>Team Nova</b></font><br><b>Security Report Analysis</b><br><b>WSO2 API Manager</b>`
	wantRef := `<a href="https://csm.example.com/cases/CASE-1">CS0001001 · WSO2-1000</a>`
	wantLabel := `Tom & Jerry <script>` // a plain label, so verbatim, not HTML-escaped
	if top != wantTop {
		t.Errorf("top text = %q, want %q", top, wantTop)
	}
	if ref != wantRef {
		t.Errorf("case reference text = %q, want %q", ref, wantRef)
	}
	if label != wantLabel {
		t.Errorf("description label = %q, want %q", label, wantLabel)
	}
}

// TestSendSecurityReportAnalysisAlert_OmitsEmptyOptionalParts verifies
// team/product are each dropped entirely when the caller doesn't supply
// one — the fixed "Security Report Analysis" label and the linked case
// reference are never omitted.
func TestSendSecurityReportAnalysisAlert_OmitsEmptyOptionalParts(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager",
		"CS0001001", "", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSecurityReportAnalysisAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	top, ref, label := cardParts(t, card)
	if want := `<b>Security Report Analysis</b>`; top != want {
		t.Errorf("top text = %q, want %q", top, want)
	}
	if want := `<a href="https://csm.example.com/cases/CASE-1">CS0001001</a>`; ref != want {
		t.Errorf("case reference text = %q, want %q", ref, want)
	}
	if label != "" {
		t.Errorf("description label = %q, want none when there is no title", label)
	}
}

func TestSendSecurityReportAnalysisAlert_RejectsEmptyCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager", "", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendCaseAcknowledgedAlert_SendsExpectedCard verifies the
// case.acknowledged card's three lines — no header, no button, no leading
// icon on any of them (dropped for width on mobile — see
// SendCaseCreatedAlert's own doc comment), and deliberately no team line
// at all, unlike the other two cards: severity alone, "<caseNumber>
// <wso2CaseID>" together, then "Ack by <name> · View case" — caseNumber
// is plain text; "View case" (not the case number) is the link, matching
// the other two cards' explicit-link-text convention.
func TestSendCaseAcknowledgedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001002", "WSO2-1001", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil (this card stays one line, no header)", card.Header)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v", card.Sections)
	}
	want := `<font color="#DC2626"><b>Critical (P1)</b></font><br>CS0001002 · WSO2-1001<br>Ack by Jane Doe · <a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

// TestSendCaseAcknowledgedAlert_OmitsEmptyWSO2CaseID verifies it's dropped
// from the line entirely (no stray separator) when the caller doesn't
// supply one.
func TestSendCaseAcknowledgedAlert_OmitsEmptyWSO2CaseID(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"High (P2)", "#EA580C", "CS0001003", "", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	text := capturedBody.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	want := `<font color="#EA580C"><b>High (P2)</b></font><br>CS0001003<br>Ack by Jane Doe · <a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if text != want {
		t.Errorf("card text = %q, want %q", text, want)
	}
}

func TestSendCaseAcknowledgedAlert_RejectsMissingRequiredArgs(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "https://example.com/cases/1", "Jane Doe"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "CS0001", "WSO2-1000", "https://example.com/cases/1", ""); err == nil {
		t.Fatal("expected error for empty acknowledgerName, got nil")
	}
}

// TestSendSeverityChangedAlert_SendsExpectedCard verifies the
// case.severity_changed card has the same headerless two-block layout as the
// created card, differing only in the severity slot. Above the divider: team
// (bold, colored), the old→new severity transition — both severities colored,
// not just the new one — and product. Below it: the case reference as a link,
// then the case subject; no button and no separate "View case" line.
func TestSendSeverityChangedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSeverityChangedAlert(context.Background(), "api-manager",
		"High (P2)", "#EA580C", "Low (P4)", "#6B7280", "CS0001002", "WSO2-1001", "WSO2 API Manager", "Gateway returns 502", "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSeverityChangedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil — a header would sit above the team line", card.Header)
	}
	top, ref, label := cardParts(t, card)
	wantTop := `<font color="#5F6368"><b>Team Nova</b></font><br><font color="#EA580C"><b>High (P2)</b></font> → <font color="#6B7280"><b>Low (P4)</b></font><br><b>WSO2 API Manager</b>`
	wantRef := `<a href="https://csm.example.com/cases/CASE-1">CS0001002 · WSO2-1001</a>`
	wantLabel := `Gateway returns 502`
	if top != wantTop {
		t.Errorf("top text = %q, want %q", top, wantTop)
	}
	if ref != wantRef {
		t.Errorf("case reference text = %q, want %q", ref, wantRef)
	}
	if label != wantLabel {
		t.Errorf("description label = %q, want %q", label, wantLabel)
	}
	if capturedBody.Thread == nil || capturedBody.Thread.ThreadKey != chatThreadKey("CS0001002") {
		t.Errorf("Thread = %+v, want ThreadKey %q — a severity change must thread the same way case.acknowledged does, including when case.created's own Chat alert was skipped (e.g. a LOW-severity case later escalated), the real bug this closes", capturedBody.Thread, chatThreadKey("CS0001002"))
	}
}

func TestSendSeverityChangedAlert_OmitsEmptyWSO2CaseIDAndTeam(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSeverityChangedAlert(context.Background(), "api-manager",
		"Medium (P3)", "#7C3AED", "Critical (P1)", "#DC2626", "CS0001003", "", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSeverityChangedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	top, ref, label := cardParts(t, card)
	if want := `<font color="#7C3AED"><b>Medium (P3)</b></font> → <font color="#DC2626"><b>Critical (P1)</b></font>`; top != want {
		t.Errorf("top text = %q, want %q", top, want)
	}
	if want := `<a href="https://csm.example.com/cases/CASE-1">CS0001003</a>`; ref != want {
		t.Errorf("case reference text = %q, want %q", ref, want)
	}
	if label != "" {
		t.Errorf("description label = %q, want none when there is no title", label)
	}
}

func TestSendSeverityChangedAlert_RejectsMissingCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendSeverityChangedAlert(context.Background(), "api-manager", "High (P2)", "#EA580C", "Low (P4)", "#6B7280", "", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendFrustrationAlert_FormatsFrustrationLevelAsADecimal guards against
// a real bug this call site had: caseAlertLine stringifies every arg before
// formatting (see its own doc comment), so passing frustrationLevel straight
// through with a %.2f verb produced a Go fmt verb-mismatch string
// (%!f(string=0.91)) in the actual Chat card instead of "0.91".
func TestSendFrustrationAlert_FormatsFrustrationLevelAsADecimal(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendFrustrationAlert(context.Background(), "api-manager",
		"CS0448647", "WSO2-1000", "WSO2 API Manager", "High urgency and explicit expression of frustration indicate dissatisfaction.",
		0.91, "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendFrustrationAlert returned error: %v", err)
	}

	got := capturedBody.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if strings.Contains(got, "%!f") {
		t.Fatalf("card text = %q, contains a Go fmt verb-mismatch marker", got)
	}
	want := `<b>WSO2 API Manager</b><br>Frustration level: <b>0.91</b><br>High urgency and explicit expression of frustration indicate dissatisfaction.<br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

// TestSendFrustrationAlert_DoesNotThread guards against a real bug: an
// earlier version set Thread.ThreadKey to the same chatThreadKey(caseNumber)
// value case.created/case.acknowledged use, which buried every frustration
// alert as a reply under that case's (possibly old, scrolled-past)
// case.created message instead of posting as its own visible, standalone
// message -- the opposite of what a frustration alert is for.
func TestSendFrustrationAlert_DoesNotThread(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendFrustrationAlert(context.Background(), "api-manager",
		"CS0448647", "WSO2-1000", "WSO2 API Manager", "reason", 0.91, "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendFrustrationAlert returned error: %v", err)
	}

	if capturedBody.Thread != nil {
		t.Errorf("Thread = %+v, want nil (frustration alerts must not thread)", capturedBody.Thread)
	}
}

// TestSendCaseCreatedAlert_UnconfiguredAudienceIsANoOpNotAnError verifies an
// audience with no matching GOOGLE_CHAT_SPACES entry is treated as a known
// configuration gap (logged, no HTTP request made) rather than an error —
// there is deliberately no "default" fallback space the way the old
// product-based mechanism had; every case.*-sending method shares this same
// sendCardToAudience behavior.
func TestSendCaseCreatedAlert_UnconfiguredAudienceIsANoOpNotAnError(t *testing.T) {
	requestMade := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "Incident Monitor", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "Polaris", "Critical (P1)", "#DC2626", "CS0001001", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v, want nil for an unconfigured audience", err)
	}
	if requestMade {
		t.Error("expected no HTTP request for an unconfigured audience")
	}
}

func TestGoogleChatClient_HasAudienceSpace(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "Incident Monitor", WebhookURL: "https://example.com"}}})

	if !c.HasAudienceSpace("Incident Monitor") {
		t.Error("HasAudienceSpace(\"Incident Monitor\") = false, want true")
	}
	if !c.HasAudienceSpace(" Incident Monitor ") {
		t.Error("HasAudienceSpace should trim whitespace")
	}
	if c.HasAudienceSpace("incident monitor") {
		t.Error("HasAudienceSpace should be case-sensitive, unlike product matching")
	}
	if c.HasAudienceSpace("Castor") {
		t.Error("HasAudienceSpace(\"Castor\") = true, want false for an unconfigured audience")
	}
}

// TestGoogleChatClient_SetTeamAudienceSpaces pins the team-sourced routing
// path (entity-service's team.chat_webhook_url, refreshed periodically —
// see cmd/server/main.go) alongside the static, config-sourced one: a team
// with no static GOOGLE_CHAT_SPACES entry resolves once its webhook is set
// dynamically, a later call fully replaces the set (not merges into it), and
// a static entry wins over a same-named dynamic one.
func TestGoogleChatClient_SetTeamAudienceSpaces(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{
		{Audience: "Incident Monitor", WebhookURL: "https://example.com/incident-monitor"},
		{Audience: "Apollo", WebhookURL: "https://example.com/apollo-static"},
	}})

	if c.HasAudienceSpace("Castor") {
		t.Fatal("HasAudienceSpace(\"Castor\") = true before any team-sourced space was set")
	}

	c.SetTeamAudienceSpaces(map[string]string{
		"Castor": "https://example.com/castor-dynamic",
		"Apollo": "https://example.com/apollo-dynamic",
	})

	if !c.HasAudienceSpace("Castor") {
		t.Error("HasAudienceSpace(\"Castor\") = false after SetTeamAudienceSpaces added it")
	}
	if url, _ := c.webhookURLFor("Apollo"); url != "https://example.com/apollo-static" {
		t.Errorf("a static GOOGLE_CHAT_SPACES entry should win over a same-named dynamic one, got %q", url)
	}
	if !c.HasAudienceSpace("Incident Monitor") {
		t.Error("the standing, config-sourced audience must stay routable alongside team-sourced ones")
	}

	// A later call replaces, it does not merge: Castor must disappear once a
	// refresh no longer reports it (e.g. the operator cleared the webhook).
	c.SetTeamAudienceSpaces(map[string]string{"Apollo": "https://example.com/apollo-dynamic"})
	if c.HasAudienceSpace("Castor") {
		t.Error("HasAudienceSpace(\"Castor\") = true after a refresh that no longer reports it")
	}
}

// TestTeamPart pins the team line: bold, one color for every team (there is
// no per-ABT palette any more — see teamTextColor), the name HTML-escaped, and
// nothing at all for a blank team so callers can skip it without a blank line.
func TestTeamPart(t *testing.T) {
	for _, team := range []string{"Sirius", "Atlas", "Castor", "Rigel", "Draco", "Phoenix", "Vega", "Apollo", "Team Nova"} {
		want := `<font color="#5F6368"><b>` + team + `</b></font>`
		if got := teamPart(team); got != want {
			t.Errorf("teamPart(%q) = %q, want %q", team, got, want)
		}
	}
	if got, want := teamPart("R&D <Core>"), `<font color="#5F6368"><b>R&amp;D &lt;Core&gt;</b></font>`; got != want {
		t.Errorf("teamPart did not escape the name: got %q, want %q", got, want)
	}
	if got := teamPart(""); got != "" {
		t.Errorf("teamPart(\"\") = %q, want empty", got)
	}
}

// TestSendCaseCreatedAlert_EveryTeamLeadsInTheSameColor is the end-to-end
// check: whichever ABT the case belongs to, the card's first line is that
// team's name, bold, in the one team color.
func TestSendCaseCreatedAlert_EveryTeamLeadsInTheSameColor(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "Incident Monitor", WebhookURL: srv.URL}}})

	for _, team := range []string{"Sirius", "Atlas", "Phoenix"} {
		if err := c.SendCaseCreatedAlert(context.Background(), "Incident Monitor",
			"High (P2)", "#F97316", "CS0378947", "LAPOSTESUB-640", "WSO2 API Manager",
			"The distributed modules use different libraries", team, "https://csm.example.com/cases/CASE-1"); err != nil {
			t.Fatalf("SendCaseCreatedAlert(%s) returned error: %v", team, err)
		}
		card := capturedBody.CardsV2[0].Card
		if card.Header != nil {
			t.Errorf("%s: Header != nil, want none: a header would be drawn above the team", team)
		}
		text := card.Sections[0].Widgets[0].TextParagraph.Text
		if first, want := strings.SplitN(text, "<br>", 2)[0], `<font color="#5F6368"><b>`+team+`</b></font>`; first != want {
			t.Errorf("%s: first line = %q, want %q", team, first, want)
		}
	}
}

// cardParts asserts the layout every team-showing case.* card shares — one
// section of [top text, divider, case reference], where top holds team,
// severity and product and the case reference is a DecoratedText whose text is
// the linked reference and whose bottom label is the description — and returns
// the top text and the reference's text and label. The one allowed variation
// is a card with nothing to put on top (no team, severity or product), which
// is just [case reference] with no divider and returns an empty top. A card
// that lost its divider, or grew a button, fails here rather than at every
// call site.
func cardParts(t *testing.T, card chatCard) (top, refText, label string) {
	t.Helper()
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil — a header would sit above the team line", card.Header)
	}
	if len(card.Sections) != 1 {
		t.Fatalf("Sections = %d, want 1", len(card.Sections))
	}
	w := card.Sections[0].Widgets
	for i, wd := range w {
		if wd.ButtonList != nil {
			t.Errorf("widget %d has a button, want none", i)
		}
	}
	var ref *chatDecoratedText
	switch len(w) {
	case 1:
		ref = w[0].DecoratedText
		if ref == nil || w[0].Divider != nil || w[0].TextParagraph != nil {
			t.Fatalf("a one-widget card must be just the case reference: %+v", w)
		}
	case 3:
		if w[0].TextParagraph == nil {
			t.Fatalf("widget 0 must be the top text paragraph: %+v", w[0])
		}
		if w[1].Divider == nil || w[1].TextParagraph != nil || w[1].DecoratedText != nil || w[1].ButtonList != nil {
			t.Fatalf("widget 1 must be a bare divider: %+v", w[1])
		}
		ref = w[2].DecoratedText
		if ref == nil || w[2].TextParagraph != nil {
			t.Fatalf("widget 2 must be the case reference (a decoratedText): %+v", w[2])
		}
		top = w[0].TextParagraph.Text
	default:
		t.Fatalf("widgets = %d, want 3 (top text, divider, case reference) or 1 (reference only): %+v", len(w), w)
	}
	if !ref.WrapText {
		t.Error("case reference WrapText = false, want true so a long reference wraps instead of truncating")
	}
	return top, ref.Text, ref.BottomLabel
}

// TestCaseCardWidgets_Layout pins the wire shape Chat needs and the edge
// case: a divider marshals as `{}` (a nil pointer would be dropped by
// omitempty, so no line would be drawn, and any field would be rejected), the
// reference's description is a `bottomLabel`, and with nothing on top there is
// neither a top text widget (an empty one is never sent) nor a divider.
func TestCaseCardWidgets_Layout(t *testing.T) {
	ref := chatCardWidget{DecoratedText: &chatDecoratedText{Text: "r", BottomLabel: "d", WrapText: true}}
	cases := []struct {
		name string
		top  []string
		want string
	}{
		{"team, severity and product on top", []string{"a", "b"}, `[{"textParagraph":{"text":"a\u003cbr\u003eb"}},{"divider":{}},{"decoratedText":{"text":"r","bottomLabel":"d","wrapText":true}}]`},
		{"nothing on top", nil, `[{"decoratedText":{"text":"r","bottomLabel":"d","wrapText":true}}]`},
	}
	for _, tc := range cases {
		b, err := json.Marshal(caseCardWidgets(tc.top, ref))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tc.want {
			t.Errorf("%s: widgets JSON = %s, want %s", tc.name, b, tc.want)
		}
	}
}

// TestCaseRefWidget pins the widget that replaced the "View case" link: the
// reference is itself the link and the main text, the description is the
// smaller bottom label — verbatim, since a label is a plain string — and with
// no link (no current caller) it degrades to the bold reference rather than an
// `<a href="">` that goes nowhere. A URL's "&" is escaped for the attribute, as
// is every other dynamic value that goes into the HTML text.
func TestCaseRefWidget(t *testing.T) {
	cases := []struct {
		name                          string
		caseNumber, wso2CaseID, title string
		link                          string
		wantText, wantLabel           string
	}{
		{"linked reference and description", "CS1", "WSO2-1", "Broken <b> & co", "https://csm.example.com/cases/1?a=1&b=2",
			`<a href="https://csm.example.com/cases/1?a=1&amp;b=2">CS1 · WSO2-1</a>`, `Broken <b> & co`},
		{"no wso2 case id, no description", "CS1", "", "", "https://csm.example.com/cases/1",
			`<a href="https://csm.example.com/cases/1">CS1</a>`, ""},
		{"no link falls back to bold", "CS1", "WSO2-1", "Subject", "",
			`<b>CS1 · WSO2-1</b>`, "Subject"},
	}
	for _, tc := range cases {
		w := caseRefWidget(tc.caseNumber, tc.wso2CaseID, tc.title, tc.link)
		if w.DecoratedText == nil {
			t.Fatalf("%s: not a decoratedText widget: %+v", tc.name, w)
		}
		if w.DecoratedText.Text != tc.wantText || w.DecoratedText.BottomLabel != tc.wantLabel || !w.DecoratedText.WrapText {
			t.Errorf("%s: got text %q label %q wrap %v, want text %q label %q wrap true",
				tc.name, w.DecoratedText.Text, w.DecoratedText.BottomLabel, w.DecoratedText.WrapText, tc.wantText, tc.wantLabel)
		}
	}
}

// TestCaseCards_HaveNoViewCaseLine guards the product decision that the
// case-created, security-report-analysis and severity-changed cards carry
// their one link on the case reference and no longer end on a "View case" line.
func TestCaseCards_HaveNoViewCaseLine(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "a", WebhookURL: srv.URL}}})
	link := "https://csm.example.com/cases/CASE-1"

	sends := map[string]func() error{
		"created": func() error {
			return c.SendCaseCreatedAlert(context.Background(), "a", "High (P2)", "#F97316", "CS1", "W-1", "Prod", "Title", "Sirius", link)
		},
		"security_report_analysis": func() error {
			return c.SendSecurityReportAnalysisAlert(context.Background(), "a", "CS1", "W-1", "Prod", "Title", "Sirius", link)
		},
		"severity_changed": func() error {
			return c.SendSeverityChangedAlert(context.Background(), "a", "High (P2)", "#F97316", "Critical (P1)", "#DC2626", "CS1", "W-1", "Prod", "Title", "Sirius", link)
		},
	}
	for name, send := range sends {
		if err := send(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		top, ref, label := cardParts(t, capturedBody.CardsV2[0].Card)
		if strings.Contains(top+ref+label, "View case") {
			t.Errorf("%s: card still has a \"View case\" line: %q %q %q", name, top, ref, label)
		}
		if want := `<a href="` + link + `">CS1 · W-1</a>`; ref != want {
			t.Errorf("%s: case reference text = %q, want the linked reference %q", name, ref, want)
		}
		if label != "Title" {
			t.Errorf("%s: description label = %q, want %q", name, label, "Title")
		}
	}
}
