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

// Package config loads alert-core-service's deployment tunables — poll cadence, retry counts, and timeouts — from a TOML file, validating every value before returning it.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultPath is the config file name used when the CONFIG_PATH env var is unset; it is expected to live at the repo or deployment root.
const DefaultPath = "config.toml"

// Config holds every deployment tunable that previously lived as a hardcoded constant, grouped by the subsystem each section configures.
type Config struct {
	Poll      PollConfig      `toml:"poll"`
	Cassandra CassandraConfig `toml:"cassandra"`
	Notify    NotifyConfig    `toml:"notify"`
	Server    ServerConfig    `toml:"server"`
	Lease     LeaseConfig     `toml:"lease"`
}

// PollConfig tunes the alert poller's backstop cadence, per-cycle worker concurrency, and how many alert ids a single cycle is allowed to process.
type PollConfig struct {
	// Interval is the backstop polling cadence; in practice a websocket ping from alert-ingestion normally wakes the poller sooner than this.
	Interval Duration `toml:"interval"`
	// Concurrency is the number of fingerprint-sharded workers handling alerts per cycle; distinct incidents run in parallel, same-fingerprint alerts stay serialized on one worker.
	Concurrency int `toml:"concurrency"`
	// ReadConcurrency bounds how many alert rows are read in parallel at the start of each poll cycle, independent of the handling worker count.
	ReadConcurrency int `toml:"read_concurrency"`
	// MaxWindow caps how many alert ids a single poll cycle processes at once, bounding memory usage under large alert bursts.
	MaxWindow int `toml:"max_window"`
	// GapTimeout bounds how long a single missing alert id blocks every id after it before this service skips it and logs loudly, instead of stalling the whole pipeline forever.
	GapTimeout Duration `toml:"gap_timeout"`
}

// LeaseConfig tunes the Cassandra-backed processor lease that elects a single active poller across replicas, so standbys never double-process the same alert.
type LeaseConfig struct {
	// TTL is how long an acquired lease stays valid without renewal; a dead leader's work resumes on a standby after at most this long.
	TTL Duration `toml:"ttl"`
	// RenewInterval is how often the leader renews its lease; it must stay well under TTL so one missed renewal never drops leadership.
	RenewInterval Duration `toml:"renew_interval"`
}

// CassandraConfig tunes startup connection retry attempts, backoff delay, connect timeout, and the per-query timeout used for every Cassandra call.
type CassandraConfig struct {
	ConnectMaxAttempts int      `toml:"connect_max_attempts"`
	ConnectBaseDelay   Duration `toml:"connect_base_delay"`
	ConnectTimeout     Duration `toml:"connect_timeout"`
	QueryTimeout       Duration `toml:"query_timeout"`
}

// NotifyConfig tunes retry attempts, backoff delay, and per-call timeout for outbound CSM and Chat webhook requests.
type NotifyConfig struct {
	MaxAttempts    int      `toml:"max_attempts"`
	RetryBaseDelay Duration `toml:"retry_base_delay"`
	HTTPTimeout    Duration `toml:"http_timeout"`
	// RetrySweepInterval is how often the poller retries incidents whose CSM or Chat notification is still outstanding; this is outage recovery, independent of poll.interval.
	RetrySweepInterval Duration `toml:"retry_sweep_interval"`
	// MaxCSMAttempts bounds how many failed CreateIncident attempts an incident absorbs before it's marked permanently failed and dropped from RetrySweep, so a payload CSM permanently rejects (or a persistently misconfigured deployment) doesn't grow incidents_processed's full-table scan cost forever.
	MaxCSMAttempts int `toml:"max_csm_attempts"`
	// ServiceCacheTTL bounds how long a label->CMDB-service-id resolution is reused before a fresh live /services/search call.
	ServiceCacheTTL Duration `toml:"service_cache_ttl"`
}

// ServerConfig tunes how long the HTTP server waits for in-flight requests to drain during a graceful shutdown before forcing the process to exit.
type ServerConfig struct {
	ShutdownGrace Duration `toml:"shutdown_grace"`
}

// Duration wraps time.Duration so human-readable TOML values like "30s" decode correctly via the standard library's time.ParseDuration function.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler, the interface the TOML decoder invokes to parse quoted duration strings like "30s" into a Duration value.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	*d = Duration(parsed)
	return nil
}

// Duration returns the wrapped value as a plain time.Duration so callers can pass it directly to standard library timer and context functions.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Load reads and validates the deployment config from path, falling back to the CONFIG_PATH env var and then DefaultPath when path is left empty.
func Load(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = DefaultPath
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

// validate rejects zero or negative tunables that would otherwise silently disable retries, skip timeouts, or leave the poller blocked forever on startup.
func (c Config) validate() error {
	switch {
	case c.Poll.Interval <= 0:
		return fmt.Errorf("poll.interval must be positive")
	case c.Poll.Concurrency <= 0:
		return fmt.Errorf("poll.concurrency must be positive")
	case c.Poll.ReadConcurrency <= 0:
		return fmt.Errorf("poll.read_concurrency must be positive")
	case c.Poll.MaxWindow <= 0:
		return fmt.Errorf("poll.max_window must be positive")
	case c.Poll.GapTimeout <= 0:
		return fmt.Errorf("poll.gap_timeout must be positive")
	case c.Lease.TTL <= 0:
		return fmt.Errorf("lease.ttl must be positive")
	case c.Lease.RenewInterval <= 0:
		return fmt.Errorf("lease.renew_interval must be positive")
	case c.Lease.RenewInterval >= c.Lease.TTL:
		return fmt.Errorf("lease.renew_interval must be less than lease.ttl")
	case c.Cassandra.ConnectMaxAttempts <= 0:
		return fmt.Errorf("cassandra.connect_max_attempts must be positive")
	case c.Cassandra.ConnectBaseDelay <= 0:
		return fmt.Errorf("cassandra.connect_base_delay must be positive")
	case c.Cassandra.ConnectTimeout <= 0:
		return fmt.Errorf("cassandra.connect_timeout must be positive")
	case c.Cassandra.QueryTimeout <= 0:
		return fmt.Errorf("cassandra.query_timeout must be positive")
	case c.Notify.MaxAttempts <= 0:
		return fmt.Errorf("notify.max_attempts must be positive")
	case c.Notify.RetryBaseDelay <= 0:
		return fmt.Errorf("notify.retry_base_delay must be positive")
	case c.Notify.HTTPTimeout <= 0:
		return fmt.Errorf("notify.http_timeout must be positive")
	case c.Notify.RetrySweepInterval <= 0:
		return fmt.Errorf("notify.retry_sweep_interval must be positive")
	case c.Notify.MaxCSMAttempts <= 0:
		return fmt.Errorf("notify.max_csm_attempts must be positive")
	case c.Notify.ServiceCacheTTL <= 0:
		return fmt.Errorf("notify.service_cache_ttl must be positive")
	case c.Server.ShutdownGrace <= 0:
		return fmt.Errorf("server.shutdown_grace must be positive")
	}
	return nil
}
