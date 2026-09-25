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

package notify

import (
	"sync"
	"time"
)

// serviceCache is a small in-memory, TTL-bounded cache mapping an alert's raw
// Service label to a CMDB service uuid already resolved via
// csm.Client.SearchServiceID, so a label doesn't pay for a live
// /services/search call on every single CSM delivery attempt. Only a
// successful resolution is ever cached: a confirmed zero-result search, or a
// transient search error, could become a different outcome on a later
// attempt, and caching either would risk pinning an incident to a stale
// wrong answer for a full TTL window.
type serviceCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]serviceCacheEntry
}

type serviceCacheEntry struct {
	id        string
	expiresAt time.Time
}

func newServiceCache(ttl time.Duration) *serviceCache {
	return &serviceCache{ttl: ttl, entries: make(map[string]serviceCacheEntry)}
}

func (c *serviceCache) get(label string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[label]
	if !ok || now.After(e.expiresAt) {
		return "", false
	}
	return e.id, true
}

func (c *serviceCache) set(label, id string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[label] = serviceCacheEntry{id: id, expiresAt: now.Add(c.ttl)}
}
