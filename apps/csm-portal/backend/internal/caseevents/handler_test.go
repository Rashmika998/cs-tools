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

package caseevents

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/eventbus"
)

func TestHandle_ValidRecord_ReturnsNil(t *testing.T) {
	h := NewHandler()
	record := eventbus.Record{
		Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"caseComment":"hello"}}`),
	}
	if err := h.Handle(t.Context(), record); err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}
}

func TestHandle_MalformedRecord_ReturnsNilNotError(t *testing.T) {
	h := NewHandler()
	record := eventbus.Record{Value: []byte("not json")}
	// A malformed record is logged and dropped, not retried — see Handle's
	// doc comment — so this must not return an error.
	if err := h.Handle(t.Context(), record); err != nil {
		t.Errorf("Handle() error = %v, want nil for a malformed record", err)
	}
}

func TestHandle_EmptyRecord_ReturnsNilNotError(t *testing.T) {
	h := NewHandler()
	if err := h.Handle(t.Context(), eventbus.Record{}); err != nil {
		t.Errorf("Handle() error = %v, want nil for an empty record", err)
	}
}
