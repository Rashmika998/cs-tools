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

package eventbus

import (
	"fmt"
	"log/slog"
)

// logDebug and logError bridge kafka.Writer/kafka.Reader's Logger/
// ErrorLogger (a bare Printf-style func) to slog, so this service's logs
// show something instead of nowhere — kafka-go, like the Kafka client used
// before it, logs nothing at all without one of these explicitly set.
//
// Logger (bridged to logDebug) is kafka-go's own designation for routine,
// non-error informational messages — join/sync/commit protocol chatter, "N
// messages written to <topic>" on every produce, and so on. Confirmed
// against the real Event Hub namespace: at Warn level this produced
// thousands of log lines a day for entirely normal operation, several a
// second during any consumer-group rebalance. ErrorLogger (bridged to
// logError) is where kafka-go actually reports problems, so routing it to
// slog.Error and Logger to slog.Debug matches kafka-go's own intended
// severity split — genuine problems stay visible by default (this
// service's log level defaults to Info — see middleware.ConfigureLogger's
// LOG_LEVEL handling), routine chatter is available when explicitly needed
// (e.g. troubleshooting a connection issue, via LOG_LEVEL=debug) without
// having to filter or classify individual message strings ourselves.
func logDebug(msg string, args ...any) {
	slog.Debug(fmt.Sprintf(msg, args...))
}

func logError(msg string, args ...any) {
	slog.Error(fmt.Sprintf(msg, args...))
}
