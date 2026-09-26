# Alert Core Service

Alert Core Service deduplicates incoming alerts into incidents and forwards them to
CSM via `csm-integration-service`, falling back to Google Chat when CSM doesn't
confirm. It reads alerts written by the separate `alert-ingestion` service from
Cassandra, folds them into incidents by fingerprint
(`source|service|metric_name|environment|unique_identifier`), and keeps retrying
notifications independently until they're actually delivered.

## How it works

- A poller discovers new alerts by comparing the `alert_seq` counter against
  its own persisted `alert_cursor` — no CDC needed. `alert-ingestion` posts to
  `POST /alert` to wake it early; a fixed interval is the backstop.
- Each alert is normalized, deduplicated by fingerprint, and folded into an
  incident. New incidents get a placeholder `incident_number` until CSM
  confirms the real one; open/closed state is synced back from CSM itself,
  never inferred locally.
- Notifications are tracked independently: an incident stays pending until
  CSM confirms, so a temporary CSM outage is retried on its own schedule
  without blocking alert ingestion. Chat is a one-time fallback while CSM
  remains unconfirmed, not a second standing obligation.

## Multi-container support

Alert Core Service is safe to run as multiple replicas (e.g. several Choreo
containers). Exactly one replica holds a Cassandra-backed, time-bounded lease
and acts as the active processor at any moment; the rest stand by. If the
active replica dies or is redeployed, a standby steals the lease once it
expires and resumes from the same durable cursor. Leadership and progress
both live in Cassandra rather than in-memory, so replicas can be added,
removed, or restarted freely -- but note that duplicate or dropped alerts are
not fully impossible: lease handoff, CSM's own dedup-by-tag lookup (used
before every incident create), and the shutdown drain sequence all narrow
those windows significantly, they don't eliminate them under every failure
mode (e.g. clock skew between replicas). See the lease and notify packages'
own doc comments for the specific tradeoffs.

## Running it

```bash
go run ./cmd/server
go build ./... && go vet ./... && go test ./...
```

Configuration lives in `config.toml` (poll cadence, retry/backoff, lease TTL)
and environment variables (`CASSANDRA_*`, `CSM_INTEGRATION_*`, `CSM_CALLER_ID`,
`CSM_UNKNOWN_SERVICE_ID`, `FALLBACK_CHAT_WEBHOOK_URLS`) — see `.env.example`.

`config.toml` itself is gitignored (see root `.gitignore`), since it's treated
as deployment config rather than source. Copy `config.toml.example` to
`config.toml` and customize as needed. Every field is required and validated
at startup (`internal/config.Config.validate`) -- there are no built-in
fallback defaults if the file is missing a value or unparsable, so
`config.toml.example`'s values are a starting point to copy and edit, not
defaults this service falls back to on its own.

```bash
cp config.toml.example config.toml
```

## Choreo Deployment

`config.toml` is not baked into the image — it's supplied at runtime via
Choreo's **Manage > Configs and Secrets > File Mount**:

1. In the component's Choreo console, go to **Manage Configs and Secrets >
   File Mount** and add a new file mount.
2. Set the file **name** to `config.toml` and paste the contents of
   `config.toml.example` (customized as needed) as the file content.
3. Mount it at the path the service reads from: the working directory root
   (so it resolves as `config.toml`), or any path if you also set the
   `CONFIG_PATH` environment variable to that path.
4. Redeploy — the poller, lease, Cassandra, notify, and server tunables all
   load from this mounted file on startup.
