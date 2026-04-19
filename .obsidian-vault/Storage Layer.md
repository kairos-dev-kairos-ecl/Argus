# Storage Layer

> Three databases, three purposes. Each has a defined role — no overlap.

## Overview

```
ClickHouse  ← all signal time-series (write-heavy, analytical queries)
PostgreSQL  ← config, auth, rules, alerts, incidents, baselines (relational)
Redis       ← ephemeral: trace correlation, dedup, rate limits, baseline cache
```

---

## ClickHouse

### Purpose
Time-series signal store. Optimised for high-throughput writes and analytical range queries.

### Connection
- File: `internal/storage/clickhouse.go`
- Driver: `github.com/ClickHouse/clickhouse-go/v2` (native protocol, port 9000)
- Config key: `ARGUS_DATABASE_CLICKHOUSE_DSN` → viper `database.clickhouse.dsn`
- Default: `localhost:9000` (no auth in dev, matches docker-compose `CLICKHOUSE_PASSWORD: ""`)

### Table: `signals`
Engine: `ReplacingMergeTree(version)` — deduplicates on `(app_id, layer, timestamp)`
Partition: `toYYYYMM(timestamp)` — monthly
TTL: 90 days from timestamp
Index granularity: 8192

All columns mirror [[Signal — ArgusSignal Schema]] fields:
- Identity: `signal_id`, `trace_id`, `span_id`, `parent_span_id`
- Source: `app_id`, `app_version`, `host_id`, `environment`, `sdk_version`
- Classification: `layer` (UInt8), `category` (String), `severity` (UInt8)
- Temporal: `timestamp` (DateTime64 ns), `duration_ms`, `ingested_at` (DateTime64 ms)
- Layer context: `ctx_l1_*` through `ctx_l10_*` (all Nullable)
- Enrichment: `enrich_baseline_deviation`, `enrich_geoip_country`, `enrich_geoip_city`, `enrich_threat_intel_hit`
- Governance: `data_classification`, `retention_policy`, `pii_detected`
- Dedup: `version` (UInt32 DEFAULT 1)

Schema DDL source: `internal/storage/schema.go` → `SignalsTableDDL` const
Applied by: `NewClickHouse()` → `applySchema()` on startup (CREATE TABLE IF NOT EXISTS)

### BatchWriter
File: `internal/storage/clickhouse.go` → `BatchWriter`

```
Write(signal) → append to buffer
  if len(buffer) >= batchSize (500)  →  flush()
  if time since last flush > 2s      →  flush() (ticker goroutine)

flush() → conn.PrepareBatch() + AppendStruct() × N + Send()
        → on failure: retry once, then drop + log
```

Metrics tracked: `storage_writes_total`, `storage_write_errors_total`, `storage_write_duration_seconds`

### Queries (from `receiver_query.go`)
- `GET /v1/signals` → keyset cursor pagination: `(timestamp, signal_id) > (cursor_ts, cursor_id)` using `FINAL`
- `GET /api/v1/traces/{traceId}` → `WHERE trace_id = ? ORDER BY timestamp` using `FINAL`
- `GET /api/v1/layers/status` → `SELECT layer, count() WHERE timestamp > now()-300s GROUP BY layer`
- `POST /api/v1/query` → raw SQL, read-only (DDL blocked), 5000 row max

### Health Check
`/health` endpoint pings ClickHouse. If unavailable → `status: degraded`.

---

## PostgreSQL

### Purpose
Relational config, auth, rules, alerts, incidents, baselines. Durable across restarts.

### Connection
- File: `internal/storage/postgres.go`
- Driver: `github.com/jackc/pgx/v5` connection pool (`pgxpool.New`)
- Config key: `ARGUS_DATABASE_POSTGRES_DSN` → viper `database.postgres.dsn`
- Migrations: `github.com/golang-migrate/migrate/v4` with embedded SQL
- Migration files: `internal/storage/migrations/` (17 files, up + down)

### Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `api_keys` | App authentication | app_id, key_hash (bcrypt), scopes, expires_at, revoked_at |
| `apps` | Registered applications | id, name, owner_id, created_at |
| `users` | Auth users | id, email, password_hash (bcrypt), role, status, mfa_secret, failed_logins, locked_until |
| `sessions` | JWT session tracking | id, user_id, refresh_token_hash, user_agent, ip, expires_at |
| `token_revocations` | Revoked JWTs | token_hash, revoked_at |
| `detection_rules` | YAML rules stored in DB | id, name, content (YAML text), enabled, created_by |
| `alerts` | Active/historical alerts | id, app_id, fingerprint (SHA256), severity, layer, category, title, signal_ids[], trace_id, status, signal_count, first_seen_at, last_seen_at, incident_id |
| `incidents` | Correlated alert clusters | id, title, description, severity, alert_ids[], trace_ids[], status, opened_at, closed_at |
| `baseline_profiles` | Statistical baselines | app_id, layer, category, mean, stddev, sample_count, computed_at |
| `routing_rules` | Alert → channel routing | id, enabled, min_severity, app_id_filter, layer_filter, channel_id (FK) |
| `notification_channels` | Notifier endpoints | id, type (slack/email/pagerduty/webhook/syslog), config (JSONB), enabled |
| `suppression_rules` | Alert suppression | id, fingerprint_pattern, expires_at |
| `audit_log` | Every user action | id, user_id, action, resource_type, resource_id, changes (JSONB), ip, ts |

### Key Operations
- `internal/auth/store_pg.go` — user/session CRUD
- `internal/alert/service.go` — alert upsert by fingerprint
- `internal/notify/router.go` — loads routing_rules every 5 min
- `internal/baseline/store.go` — persists computed profiles

---

## Redis

### Purpose
Ephemeral fast-path data. All keys have TTLs — nothing permanent.

### Connection
- File: `internal/storage/redis.go`
- Driver: `github.com/redis/go-redis/v9`
- Config key: `ARGUS_REDIS_ADDR` → viper `redis.addr`
- Default: `localhost:6379`
- Memory limit: 512MB `allkeys-lru` eviction (docker-compose)

### Key Patterns

| Key Pattern | Type | TTL | Used By | Purpose |
|-------------|------|-----|---------|---------|
| `trace:{trace_id}` | HASH (signal_id → timestamp) | 15 min | CorrelationTagger | Trace signal grouping |
| `trace:{trace_id}` | ZSET (score=UnixMilli) | 30 sec | CorrelationTagger | Sorted span ordering |
| `ratelimit:{key}` | STRING (counter) | window | AuthValidator | Rate limiting per app |
| `dedup:{signal_id}` | STRING (SET NX EX) | 15 min | receiver_http | Signal deduplication |
| `dedup:{fingerprint}` | STRING (SET NX EX) | 10 min | AlertRouter | Alert deduplication |
| `det:t3:{rule}:{app}:{cat}` | ZSET (score=ts) | rule window | Tier3 detection | Temporal frequency counting |
| `baseline:{app}:{layer}:{cat}` | STRING (JSON profile) | 5 min | BaselineScorer | Cached baseline profile |
| `geoip:{ip}` | STRING (JSON geo) | 24 hr | GeoIPEnricher | GeoIP cache |

### Key Operations
- `SetTraceSignal(traceID, signalID, ts)` → ZADD + EXPIRE
- `GetTraceSignals(traceID)` → ZRANGE
- `IncrRateLimit(key, window)` → INCR + EXPIRE if new
- `CheckDedup(signalID)` → SET NX EX → bool
- `ZAddTemporal(key, ts, member)` → ZADD + ZREMRANGEBYSCORE (cleanup old) + ZCARD

---

## Related Files

| File | Role |
|------|------|
| `internal/storage/schema.go` | ClickHouse table DDL |
| `internal/storage/clickhouse.go` | ClickHouse client + BatchWriter |
| `internal/storage/postgres.go` | PostgreSQL pool + migration runner |
| `internal/storage/redis.go` | Redis operations |
| `internal/storage/migrations/*.sql` | 17 up/down migration files |
| `docker-compose.yml` | Runs all 3 databases |
