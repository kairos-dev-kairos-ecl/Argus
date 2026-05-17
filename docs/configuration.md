# Configuration Reference — Argus XDR

Argus is configured through a YAML file (`argus.yaml` by default) with full environment-variable override support. This document covers every configuration option, its default, and its effect.

---

## Config File Location

By default, Argus looks for `argus.yaml` in the current working directory. Override with the `--config` flag:

```bash
./argus server --config /etc/argus/argus.yaml
```

---

## Full Config Reference

```yaml
# argus.yaml

server:
  # gRPC server listen address
  grpc_addr: "localhost:5001"
  # HTTP server listen address (REST API + dashboard)
  http_addr: "localhost:8080"
  # HTTP read/write timeout
  read_timeout:  "30s"
  write_timeout: "60s"
  # Enable development mode: in-memory storage, hardcoded auth key "dev-key-argus"
  # NEVER use dev mode in production
  dev: false

storage:
  clickhouse:
    # Native protocol address (not HTTP)
    addr:     "localhost:9000"
    database: "default"
    username: "default"
    password: ""
    # Connection pool
    max_open_conns:  10
    max_idle_conns:  5
    conn_max_lifetime: "10m"
    # Query/dial timeouts
    dial_timeout:  "5s"
    read_timeout:  "60s"
    write_timeout: "10s"
    # Compression (lz4 or zstd recommended for production)
    compression: "lz4"

  postgres:
    dsn: "postgres://argus:argus@localhost:5432/argus?sslmode=disable"
    max_conns:  20
    min_conns:   5
    max_conn_lifetime: "30m"
    max_conn_idle_time: "5m"

  redis:
    addr:     "localhost:6379"
    password: ""
    db:       0
    # Connection pool
    pool_size:    10
    min_idle_conns: 2
    # Timeouts
    dial_timeout:  "5s"
    read_timeout:  "3s"
    write_timeout: "3s"

ingest:
  # Internal signal queue capacity
  queue_capacity: 100000
  # ClickHouse batch writer settings
  batch_size:     500
  flush_interval: "2s"
  # Max message sizes
  max_grpc_message_bytes: 10485760   # 10MB
  max_http_body_bytes:    4194304    # 4MB
  # API key cache TTL in Redis
  auth_cache_ttl: "5m"
  # Worker pool size (0 = GOMAXPROCS × 2)
  worker_count: 0

detection:
  # Directory containing YAML rule files
  rules_dir: "internal/rules/built-in"
  # How often to reload rules from disk
  reload_interval: "60s"
  # Enable/disable detection engine entirely
  enabled: true

baseline:
  # Async computation interval
  compute_interval: "10m"
  # Minimum samples before a baseline is computed
  min_samples: 100
  # Redis TTL for cached profiles
  cache_ttl: "5m"
  # PostgreSQL retention for stored profiles
  profile_retention: "30d"

auth:
  # JWT signing key (RS256 — provide PEM-encoded private key path)
  jwt_private_key_path:  "secrets/jwt_private.pem"
  jwt_public_key_path:   "secrets/jwt_public.pem"
  # Token lifetimes
  access_token_ttl:  "1h"
  refresh_token_ttl: "7d"
  # CSRF
  csrf_enabled: true
  csrf_cookie_secure: false   # Set true in production (HTTPS only)
  # MFA
  mfa_issuer: "ArgusXDR"
  # Password policy
  min_password_length: 12
  hibp_check_enabled:  true   # HIBP breach check (fail-open on network error)

notify:
  # Notification dispatcher settings
  slack:
    webhook_url:  ""
    channel:      "#argus-alerts"
    enabled:      false
  pagerduty:
    routing_key:  ""
    enabled:      false
  email:
    smtp_host:    "localhost"
    smtp_port:    25
    from_address: "argus@example.com"
    enabled:      false
  webhook:
    url:          ""
    timeout:      "10s"
    enabled:      false

logging:
  # Log level: debug, info, warn, error
  level:  "info"
  # Output format: json (production) or console (development)
  format: "json"
  # Destination: stdout or a file path
  output: "stdout"

metrics:
  # Prometheus metrics endpoint
  enabled: true
  path:    "/metrics"
```

---

## Environment Variable Overrides

Every config key can be overridden with an environment variable prefixed `ARGUS_`. Key separators become underscores:

| Config key | Environment variable |
|------------|---------------------|
| `server.http_addr` | `ARGUS_SERVER_HTTP_ADDR` |
| `server.grpc_addr` | `ARGUS_SERVER_GRPC_ADDR` |
| `storage.clickhouse.addr` | `ARGUS_STORAGE_CLICKHOUSE_ADDR` |
| `storage.clickhouse.password` | `ARGUS_STORAGE_CLICKHOUSE_PASSWORD` |
| `storage.postgres.dsn` | `ARGUS_STORAGE_POSTGRES_DSN` |
| `storage.redis.addr` | `ARGUS_STORAGE_REDIS_ADDR` |
| `storage.redis.password` | `ARGUS_STORAGE_REDIS_PASSWORD` |
| `ingest.queue_capacity` | `ARGUS_INGEST_QUEUE_CAPACITY` |
| `ingest.batch_size` | `ARGUS_INGEST_BATCH_SIZE` |
| `auth.jwt_private_key_path` | `ARGUS_AUTH_JWT_PRIVATE_KEY_PATH` |
| `logging.level` | `ARGUS_LOGGING_LEVEL` |

Environment variables take precedence over the config file.

---

## Secrets Management

### Development

For local development, put credentials in `argus.yaml.local` (gitignored):

```yaml
# argus.yaml.local — NOT committed to git
storage:
  postgres:
    dsn: "postgres://argus:dev-password@localhost:5432/argus?sslmode=disable"
  redis:
    password: "dev-redis-password"
auth:
  jwt_private_key_path: "secrets/jwt_private.pem"
```

### Production

Use environment variables injected by your secrets manager (Vault, AWS Secrets Manager, Kubernetes Secrets). Never put production credentials in a committed config file.

JWT keys should be generated once and stored in your secrets manager:

```bash
# Generate RS256 key pair
openssl genrsa -out secrets/jwt_private.pem 2048
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
chmod 600 secrets/jwt_private.pem
```

The `secrets/` directory is gitignored.

---

## Docker Compose Configuration

The included `docker-compose.yml` pre-configures the full stack for development. Key overrides:

```yaml
services:
  argus-server:
    environment:
      - ARGUS_STORAGE_CLICKHOUSE_ADDR=clickhouse:9000
      - ARGUS_STORAGE_POSTGRES_DSN=postgres://argus:argus@postgres:5432/argus
      - ARGUS_STORAGE_REDIS_ADDR=redis:6379
      - ARGUS_LOGGING_LEVEL=debug
```

---

## Health Check Endpoints

| Endpoint | Auth | Returns |
|----------|------|---------|
| `GET /health` | None | `{"status":"healthy", "clickhouse":{...}, "postgres":{...}, "redis":{...}}` |
| `GET /metrics` | None | Prometheus text format |

`/health` returns HTTP 200 if all components are reachable, HTTP 503 if any component is degraded. Use this as your load balancer health check.

A degraded component produces output like:

```json
{
  "status": "degraded",
  "clickhouse": {"status": "ok", "latency_ms": 54},
  "postgres":   {"status": "ok", "latency_ms": 1},
  "redis":      {"status": "error", "error": "dial tcp: connection refused"}
}
```

Redis degradation is non-fatal — ingest continues. ClickHouse or PostgreSQL degradation means signals cannot be persisted or auth cannot be validated.

---

## Performance Tuning

### Ingest throughput

Default settings handle ~10K signals/sec sustained on modern hardware. To increase throughput:

```yaml
ingest:
  queue_capacity: 200000   # Increase buffer for burst traffic
  batch_size: 1000         # Larger batches → fewer ClickHouse round-trips
  flush_interval: "5s"     # Longer interval → larger average batch
  worker_count: 16         # More pipeline workers (watch CPU)
```

### ClickHouse connection pool

For high ingestion rates, increase the connection pool:

```yaml
storage:
  clickhouse:
    max_open_conns: 20
    max_idle_conns: 10
```

### Redis TTL tuning

Shorten TTLs to reduce Redis memory usage on high-cardinality trace workloads:

```yaml
ingest:
  auth_cache_ttl: "2m"   # More frequent auth validation, less cache memory
```

---

## CLI Flags

Most config options can also be passed as CLI flags. Flags override environment variables which override the config file.

```bash
./argus server \
  --config argus.yaml \
  --http-addr 0.0.0.0:8080 \
  --grpc-addr 0.0.0.0:5001 \
  --clickhouse-dsn clickhouse://localhost:9000/default \
  --postgres-dsn "postgres://argus:argus@localhost:5432/argus" \
  --queue-capacity 100000 \
  --batch-size 500 \
  --batch-interval 2s \
  --log-level info \
  --log-format json
```

Run `./argus server --help` for the complete flag list.
