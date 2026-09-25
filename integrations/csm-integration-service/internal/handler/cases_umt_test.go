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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupCase(t *testing.T) {
	t.Run("rejects missing number", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodGet, "/cases/lookup", nil)
		w := httptest.NewRecorder()
		h.LookupCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgNumberRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects blank number", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodGet, "/cases/lookup?number=%20%20", nil)
		w := httptest.NewRecorder()
		h.LookupCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgNumberRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("builds an exact-match filter and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[{"id":"11111111-1111-1111-1111-111111111111","number":"CS0012345"}],"total":1,"limit":0,"offset":0}`), nil
			},
		}
		h := NewCaseHandler(client, "")
		r := httptest.NewRequest(http.MethodGet, "/cases/lookup?number=CS0012345", nil)
		w := httptest.NewRecorder()
		h.LookupCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var sent struct {
			Filters struct {
				Filters []struct {
					Field  string   `json:"field"`
					Op     string   `json:"op"`
					Values []string `json:"values"`
				} `json:"filters"`
			} `json:"filters"`
		}
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("decode captured body: %v; raw: %s", err, capturedBody)
		}
		if len(sent.Filters.Filters) != 1 {
			t.Fatalf("filters = %v, want exactly 1", sent.Filters.Filters)
		}
		f := sent.Filters.Filters[0]
		if f.Field != "number" || f.Op != "eq" || len(f.Values) != 1 || f.Values[0] != "CS0012345" {
			t.Errorf("filter = %+v, want field=number op=eq values=[CS0012345]", f)
		}

		resp := decodeJSON[map[string]any](t, w)
		cases, _ := resp["cases"].([]any)
		if len(cases) != 1 {
			t.Errorf("cases = %v, want 1 entry", resp["cases"])
		}
	})

	t.Run("returns empty result set verbatim when no case matches", func(t *testing.T) {
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"cases":[],"total":0,"limit":0,"offset":0}`), nil
			},
		}
		h := NewCaseHandler(client, "")
		r := httptest.NewRequest(http.MethodGet, "/cases/lookup?number=CS9999999", nil)
		w := httptest.NewRecorder()
		h.LookupCase(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		cases, _ := resp["cases"].([]any)
		if len(cases) != 0 {
			t.Errorf("cases = %v, want empty", resp["cases"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to look up case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "")
				r := httptest.NewRequest(http.MethodGet, "/cases/lookup?number=CS0012345", nil)
				w := httptest.NewRecorder()
				h.LookupCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAddCaseLabel(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases//tags", strings.NewReader(`{"label":"urgent"}`))
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/case-42/tags", strings.NewReader(`{"label":"urgent"}`))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`not-json`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", nil)
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing label", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgLabelRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects blank label", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"   "}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgLabelRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("injects the configured actorEmail and ignores a caller-supplied one", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			addCaseTagFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"id":"tag-1","label":"urgent"}`), nil
			},
		}
		h := NewCaseHandler(client, "svc@example.com")
		reqBody := `{"label":"urgent","actorEmail":"attacker@example.com"}`
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseLabel(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if capturedCaseID != caseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, caseID)
		}

		var sent struct {
			Label      string `json:"label"`
			ActorEmail string `json:"actorEmail"`
		}
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("decode captured body: %v; raw: %s", err, capturedBody)
		}
		if sent.Label != "urgent" {
			t.Errorf("label = %q, want %q", sent.Label, "urgent")
		}
		if sent.ActorEmail != "svc@example.com" {
			t.Errorf("actorEmail = %q, want the configured service actor email, never the caller-supplied one", sent.ActorEmail)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["label"] != "urgent" {
			t.Errorf("label = %v, want %v", resp["label"], "urgent")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to add case label.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					addCaseTagFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "svc@example.com")
				r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"urgent"}`))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.AddCaseLabel(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestConcludeCase(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases//conclude", strings.NewReader(`{"comment":"done"}`))
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/case-42/conclude", strings.NewReader(`{"comment":"done"}`))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(`not-json`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", nil)
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing comment", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(`{"updateLevel":"1.2.3"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgCommentRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("both legs succeed: reports both outcomes with 200", func(t *testing.T) {
		var patchBody, commentBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				if id != caseID {
					t.Errorf("PatchCase caseID = %q, want %q", id, caseID)
				}
				patchBody = body
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + caseID + `"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				if id != caseID {
					t.Errorf("CreateCaseComment caseID = %q, want %q", id, caseID)
				}
				commentBody = body
				return []byte(`{"message":"Comment created successfully","comment":{"id":"c-1"}}`), nil
			},
		}
		h := NewCaseHandler(client, "svc@example.com")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(`{"comment":"Fix issued in 1.2.3.","updateLevel":"1.2.3"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		var sentPatch struct {
			MarkFixIssued bool `json:"markFixIssued"`
		}
		if err := json.Unmarshal(patchBody, &sentPatch); err != nil {
			t.Fatalf("decode patch body: %v; raw: %s", err, patchBody)
		}
		if !sentPatch.MarkFixIssued {
			t.Errorf("markFixIssued = %v, want true", sentPatch.MarkFixIssued)
		}

		var sentComment struct {
			Type       string `json:"type"`
			Content    string `json:"content"`
			ActorEmail string `json:"actorEmail"`
		}
		if err := json.Unmarshal(commentBody, &sentComment); err != nil {
			t.Fatalf("decode comment body: %v; raw: %s", err, commentBody)
		}
		if sentComment.Type != "comment" {
			t.Errorf("comment type = %q, want %q", sentComment.Type, "comment")
		}
		if !strings.Contains(sentComment.Content, "1.2.3") || !strings.Contains(sentComment.Content, "Fix issued in 1.2.3.") {
			t.Errorf("comment content = %q, want it to fold in the updateLevel and the original comment", sentComment.Content)
		}
		// ConcludeCase builds and sends this leg's body directly to the entity
		// client -- it does not go through CreateCaseComment's own
		// actorEmail-injection logic, so this leg must inject the configured
		// M2M actor identity itself, the same way AddCaseLabel does.
		if sentComment.ActorEmail != "svc@example.com" {
			t.Errorf("comment actorEmail = %q, want the configured service actor email %q", sentComment.ActorEmail, "svc@example.com")
		}

		resp := decodeJSON[map[string]map[string]any](t, w)
		if resp["markFixIssued"]["success"] != true {
			t.Errorf("markFixIssued.success = %v, want true", resp["markFixIssued"]["success"])
		}
		if resp["comment"]["success"] != true {
			t.Errorf("comment.success = %v, want true", resp["comment"]["success"])
		}
		if _, hasErr := resp["markFixIssued"]["error"]; hasErr {
			t.Errorf("markFixIssued.error = %v, want absent on success", resp["markFixIssued"]["error"])
		}
	})

	t.Run("markFixIssued succeeds, comment leg upstream error: still 200 reporting both independently", func(t *testing.T) {
		patchCalled := false
		commentCalled := false
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				patchCalled = true
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + caseID + `"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				commentCalled = true
				return nil, upstreamErrors("")[0].err // apierror 401
			},
		}
		h := NewCaseHandler(client, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(`{"comment":"Fix issued."}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if !patchCalled {
			t.Error("PatchCase was not called")
		}
		if !commentCalled {
			t.Error("CreateCaseComment was not called")
		}

		resp := decodeJSON[map[string]map[string]any](t, w)
		if resp["markFixIssued"]["success"] != true {
			t.Errorf("markFixIssued.success = %v, want true (this leg is expected to succeed)", resp["markFixIssued"]["success"])
		}
		if resp["comment"]["success"] != false {
			t.Errorf("comment.success = %v, want false (upstream returned an error for this leg)", resp["comment"]["success"])
		}
		if resp["comment"]["error"] != ErrMsgUnauthorized {
			t.Errorf("comment.error = %v, want %v", resp["comment"]["error"], ErrMsgUnauthorized)
		}
	})

	t.Run("both legs fail: still 200, both reported as failures", func(t *testing.T) {
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, upstreamErrors("")[0].err
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				return nil, upstreamErrors("")[0].err
			},
		}
		h := NewCaseHandler(client, "")
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/conclude", strings.NewReader(`{"comment":"Fix issued."}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.ConcludeCase(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]map[string]any](t, w)
		if resp["markFixIssued"]["success"] != false {
			t.Errorf("markFixIssued.success = %v, want false", resp["markFixIssued"]["success"])
		}
		if resp["comment"]["success"] != false {
			t.Errorf("comment.success = %v, want false", resp["comment"]["success"])
		}
	})
}
