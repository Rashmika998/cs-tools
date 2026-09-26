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

import "time"

// stuckTracker tracks how long the window's head alert id has been unready, backing GapTimeout; not safe for concurrent use.
type stuckTracker struct {
	at    int64
	since time.Time
}

// observe records whether base is the stuck head and reports if it's been stuck long enough (gapTimeout) to skip.
func (t *stuckTracker) observe(base int64, headBlocked bool, now time.Time, gapTimeout time.Duration) (skip bool) {
	if !headBlocked {
		if t.at == base {
			t.at = 0 // made progress past what used to be the stuck id.
		}
		return false
	}
	if t.at != base {
		t.at = base
		t.since = now
		return false
	}
	if gapTimeout <= 0 || now.Sub(t.since) < gapTimeout {
		return false
	}
	t.at = 0
	return true
}
