# Alert Core Service

Alert Core Service deduplicates incoming alerts into incidents and forwards them to
CSM, falling back to Google Chat when CSM doesn't confirm. It reads alerts
written by the separate `alert-ingestion` service from Cassandra, folds them
into incidents by fingerprint (`source|service|metric_name|environment`), and
keeps retrying notifications independently until they're actually delivered.

## How it works

- A poller discovers new alerts by comparing the `alert_seq` counter against
  its own persisted `alert_cursor` — no CDC needed. `alert-ingestion` pings
  `POST /ping` to wake it early; a fixed interval is the backstop.
- Each alert is normalized, deduplicated by fingerprint, and folded into an
  incident. New incidents get a placeholder `incident_number` until CSM
  confirms the real one.
- Notifications are tracked independently: an incident stays pending until
  CSM confirms **and** Chat has delivered, so a temporary outage on either
  side is retried on its own schedule without blocking alert ingestion.

## Multi-container support

Alert Core Service is safe to run as multiple replicas (e.g. several Choreo
containers) without any special configuration. Exactly one replica holds a
Cassandra-backed, time-bounded lease and acts as the active processor at any
moment; the rest stand by. If the active replica dies or is redeployed, a
standby steals the lease once it expires and resumes from the same durable
cursor — no alert is duplicated or dropped, and failover is bounded by the
lease TTL rather than requiring manual intervention. Because leadership and
progress both live in Cassandra rather than in-memory, replicas can be added,
removed, or restarted freely.

## Running it

```bash
go run ./cmd/server
go build ./... && go vet ./... && go test ./...
```

Configuration lives in `config.toml` (poll cadence, retry/backoff, lease TTL)
and environment variables (`CASSANDRA_*`, `CSM_WEBHOOK_URL`,
`FALLBACK_CHAT_WEBHOOK_URLS`) — see `.env.example`.

`config.toml` itself is gitignored (see root `.gitignore`), since it's treated
as deployment config rather than source. Copy `config.toml.example` to
`config.toml` and customize as needed — the example file's values are also
the defaults the service falls back to.

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
