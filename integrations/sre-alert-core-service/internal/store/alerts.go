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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gocql/gocql"

	"alert-core-service/internal/model"
)

// ErrMalformedAlert marks a decode failure on a stored alert payload as permanent, distinguishing it from transient read errors that should be retried.
var ErrMalformedAlert = errors.New("malformed alert payload")

// AlertRepo reads the alerts table; alert-ingestion writes it, alert-core-service only ever reads it.
type AlertRepo struct {
	session *gocql.Session
}

// NewAlertRepo returns a ready repo; nothing to seed since alert-ingestion owns writes.
func NewAlertRepo(session *gocql.Session) *AlertRepo {
	return &AlertRepo{session: session}
}

// Get reads the alert stored under id.
func (r *AlertRepo) Get(ctx context.Context, id string) (model.Alert, error) {
	var payload string
	if err := r.session.Query(
		`SELECT alert FROM alerts WHERE id = ?`, id,
	).WithContext(ctx).Scan(&payload); err != nil {
		return model.Alert{}, fmt.Errorf("read alert %s: %w", id, err)
	}
	var a model.Alert
	if err := json.Unmarshal([]byte(payload), &a); err != nil {
		return model.Alert{}, fmt.Errorf("decode alert %s: %w: %v", id, ErrMalformedAlert, err)
	}
	return a, nil
}
