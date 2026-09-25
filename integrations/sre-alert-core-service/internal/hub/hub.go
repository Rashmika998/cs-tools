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

// Package hub is alert-core-service's websocket endpoint: alert-ingestion pings it to wake the poller early; poller does the real work.
package hub

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

// waker is satisfied by *poll.Poller; kept narrow so hub doesn't need to import poll.
type waker interface {
	Wake()
}

// Hub upgrades publisher connections and wakes the poller on every frame.
type Hub struct {
	logger   *slog.Logger
	poller   waker
	upgrader websocket.Upgrader
}

// New returns a ready hub.
func New(logger *slog.Logger, p waker) *Hub {
	return &Hub{
		logger: logger,
		poller: p,
		// alert-ingestion is a trusted in-mesh caller, not a browser; accept any origin.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// ServePing upgrades a pinger and wakes the poller on every frame until the connection closes.
func (h *Hub) ServePing(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ping upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	h.logger.Info("alert-ingestion connected")
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.logger.Info("alert-ingestion disconnected", "error", err)
			return
		}
		h.poller.Wake()
	}
}
