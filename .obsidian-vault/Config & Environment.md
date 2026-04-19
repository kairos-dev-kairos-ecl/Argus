# Config & Environment

> Viper + environment variables. Zero config files needed for dev.

## Viper Setup

File: `cmd/argus/main.go` → `cmd/argus/api.go`

Viper prefix: `ARGUS` (case-insensitive)

Binding order (highest → lowest priority):
1. Command-line flags (`cobra.Flags()` → `viper.BindPFlag()`)
2. Environment variables (`ARGUS_*` → `viper.BindEnv()`)
3. Config file (`argus.yaml` in `/etc/argus`, `.`, `./config`)
4. Defaults (hardcoded in flag definitions)

---

## ARGUS_* Environment Variables

### Server

| Var | Viper Key | Default | Type |
|-----|-----------|---------|------|
| `ARGUS_SERVER_HTTP_ADDR` | `server.http.addr` | `localhost:8080` | string |
| `ARGUS_SERVER_GRPC_ADDR` | `server.grpc.addr` | `localhost:5001` | string |

### Database

| Var | Viper Key | Default | Type | Notes |
|-----|-----------|---------|------|-------|
| `ARGUS_DATABASE_CLICKHOUSE_DSN` | `database.clickhouse.dsn` | `localhost:9000` | string | Host:port or `clickhouse://[user:pass@]host:port` |
| `ARGUS_DATABASE_POSTGRES_DSN` | `database.postgres.dsn` | *(optional)* | string | `postgresql://user:pass@host:port/dbname?sslmode=disable` |

### Cache & Storage

| Var | Viper Key | Default | Type |
|-----|-----------|---------|------|
| `ARGUS_REDIS_ADDR` | `redis.addr` | `localhost:6379` | string |
| `ARGUS_INGEST_QUEUE_CAPACITY` | `ingest.queue.capacity` | `100000` | int |
| `ARGUS_STORAGE_BATCH_SIZE` | `storage.batch.size` | `500` | int |
| `ARGUS_STORAGE_BATCH_INTERVAL` | `storage.batch.interval` | `2s` | duration |

### Detection

| Var | Viper Key | Default | Type |
|-----|-----------|---------|------|
| `ARGUS_DETECTION_RULES_DIR` | `detection.rules_dir` | `internal/rules/built-in` | string |
| `ARGUS_BASELINE_HISTORY_WINDOW` | `baseline.history_window` | `24h` | duration |
| `ARGUS_BASELINE_COMPUTE_INTERVAL` | `baseline.compute_interval` | `10m` | duration |
| `ARGUS_BASELINE_MIN_SAMPLES` | `baseline.min_samples` | `100` | int |

### Logging

| Var | Viper Key | Default | Type |
|-----|-----------|---------|------|
| `ARGUS_LOGGING_DEV` | `logging.dev` | `false` | bool |
| `ARGUS_LOGGING_LEVEL` | `logging.level` | `info` | string |

---

## Cobra Flags (CLI)

The `argus api` subcommand accepts these flags (all optional):

```bash
argus api \
  --http-addr=0.0.0.0:8080 \
  --grpc-addr=0.0.0.0:5001 \
  --clickhouse-dsn=localhost:9000 \
  --postgres-dsn="postgresql://argus:argus@localhost:5432/argus?sslmode=disable" \
  --redis-addr=localhost:6379 \
  --ingest-queue-capacity=100000 \
  --storage-batch-size=500 \
  --storage-batch-interval=2s \
  --rules-dir=internal/rules/built-in \
  --dev \
  --log-level=debug
```

All flags are bound to viper keys via `viper.BindPFlag()` in `api.go`.

---

## Config File (Optional)

File: `argus.yaml` (searched in `/etc/argus`, `.`, `./config`)

Example:
```yaml
server:
  http:
    addr: "0.0.0.0:8080"
  grpc:
    addr: "0.0.0.0:5001"

database:
  clickhouse:
    dsn: "localhost:9000"
  postgres:
    dsn: "postgresql://argus:argus@localhost:5432/argus?sslmode=disable"

redis:
  addr: "localhost:6379"

ingest:
  queue:
    capacity: 100000

storage:
  batch:
    size: 500
    interval: "2s"

detection:
  rules_dir: "internal/rules/built-in"

logging:
  dev: false
  level: "info"
```

---

## Development Workflow

### Local (no config file)

```bash
# Terminal 1: ClickHouse + PostgreSQL + Redis
docker-compose -f deployments/docker-compose.yml up

# Terminal 2: API server (uses env vars or defaults)
export ARGUS_DATABASE_POSTGRES_DSN="postgresql://argus:argus@localhost:5432/argus?sslmode=disable"
./bin/argus-api api --dev

# Or via Makefile (Makefile sets env vars)
make run-api
```

### Docker

```yaml
argus-server:
  environment:
    ARGUS_SERVER_HTTP_ADDR: "0.0.0.0:8080"
    ARGUS_SERVER_GRPC_ADDR: "0.0.0.0:5001"
    ARGUS_DATABASE_CLICKHOUSE_DSN: "clickhouse:9000"
    ARGUS_DATABASE_POSTGRES_DSN: "postgresql://argus:argus@postgres:5432/argus?sslmode=disable"
    ARGUS_REDIS_ADDR: "redis:6379"
    ARGUS_LOGGING_LEVEL: "debug"
```

The Dockerfile (line 94) runs:
```bash
ENTRYPOINT ["./argus-api", "api", "--dev"]
```

---

## Graceful Degradation (P3 Pattern)

All external services are optional. API starts and returns:

```json
{
  "status": "degraded",
  "components": {
    "clickhouse": {
      "status": "unhealthy",
      "error": "not configured | connection refused | auth failed"
    },
    "postgres": {
      "status": "healthy"
    },
    "redis": {
      "status": "healthy"
    }
  }
}
```

Query endpoints return `503 Storage Unavailable` if ClickHouse is down.
Auth/alert features are disabled if PostgreSQL is down.
Dedup and Tier3 detection are disabled if Redis is down.

---

## Related Files

| File | Role |
|------|------|
| `cmd/argus/main.go` | Viper init + cobra root |
| `cmd/argus/api.go` | `argus api` subcommand, Viper bindings, service init |
| `cmd/argus/utils.go` | `newLogger()`, `maskDSN()` |
| `docker-compose.yml` | Environment variable examples |
| `Makefile` | `make run-api` sets env vars |
| `internal/baseline/config.go` | Baseline-specific config struct |
| `internal/pipeline/config.go` | Pipeline-specific config struct |
