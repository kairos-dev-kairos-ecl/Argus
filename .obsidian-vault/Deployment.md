# Deployment

> Docker Compose for dev, multi-stage Dockerfile for production, Makefile orchestration.

## Quick Start

```bash
# Clone + build
git clone https://github.com/argusxdr/argus.git
cd ArgusXDR

# Build Go binary
make build

# Start services (ClickHouse, PostgreSQL, Redis)
make docker-up

# Start API server (Terminal 2)
make run-api

# Run test harness (Terminal 3, after API ready)
cd test_harness
python qwen_llama_api.py

# Validate signals
python validate_signals.py

# View dashboard
open http://localhost:3000
```

---

## Docker Compose (`docker-compose.yml`)

Three-service stack: ClickHouse + PostgreSQL + Redis.

**Note:** No `argus-server` service in current docker-compose.yml. The API binary runs locally via `make run-api`.

### ClickHouse

```yaml
clickhouse:
  image: clickhouse/clickhouse-server:24-alpine
  container_name: argus-clickhouse
  ports:
    - "9000:9000"   # Native protocol
    - "8123:8123"   # HTTP
  environment:
    CLICKHOUSE_DB: default
    CLICKHOUSE_USER: default
    CLICKHOUSE_PASSWORD: ""  # No auth in dev
    CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
  volumes:
    - clickhouse_data:/var/lib/clickhouse
  healthcheck:
    test: ["CMD", "clickhouse-client", "--query", "SELECT 1"]
    interval: 5s
    timeout: 5s
    retries: 20
```

### PostgreSQL

```yaml
postgres:
  image: postgres:16-alpine
  container_name: argus-postgres
  environment:
    POSTGRES_DB: argus
    POSTGRES_USER: argus
    POSTGRES_PASSWORD: argus
    POSTGRES_INITDB_ARGS: "-c log_statement=none"
  ports:
    - "5432:5432"
  volumes:
    - postgres_data:/var/lib/postgresql/data
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U argus -d argus"]
```

### Redis

```yaml
redis:
  image: redis:7.2-alpine
  container_name: argus-redis
  command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
  ports:
    - "6379:6379"
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
```

All services use `networks: [argus-network]` (172.20.0.0/16) for inter-service communication.

---

## Dockerfile

File: `Dockerfile` (root directory)

Multi-stage build:

### Stage 1: Builder

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags "-s -w" \
  -o argus-api ./cmd/argus
```

- `CGO_ENABLED=0` — pure Go binary (no libc dependency)
- `-ldflags "-s -w"` — strip symbols/debug info (smaller binary)

### Stage 2: Runtime

```dockerfile
FROM alpine:3.19
RUN apk add --no-cache curl ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/argus-api .

# Copy built-in rules (needed by detection engine)
COPY internal/rules/built-in ./internal/rules/built-in

# Expose ports
EXPOSE 8080 5001

# Health check
HEALTHCHECK --interval=10s --timeout=5s --retries=10 \
  CMD curl -f http://localhost:8080/health || exit 1

# Default command
ENTRYPOINT ["./argus-api", "api", "--dev"]
```

Output image: ~50MB (static binary + Alpine + rules + certs)

---

## Makefile

File: `Makefile` (root directory)

Key targets:

```bash
make help              # Show all targets

make build             # Build API binary (./bin/argus-api)
make clean             # Remove ./bin/

make docker-up         # Start ClickHouse, PostgreSQL, Redis
make docker-down       # Stop services
make docker-logs       # Stream logs

make run-api           # Build + start API server locally
                       # Sets ARGUS_* env vars
                       # Connects to services from docker-up

make proto-generate    # Generate proto stubs (buf generate)
make proto-validate    # Format + lint + breaking check

make harness-llama     # Run test harness (test_harness/qwen_llama_api.py)
make validate          # Run validation suite (test_harness/validate_signals.py)

make lint              # Run golangci-lint
make fmt               # Format Go code
make test              # Run all Go tests
```

---

## Environment Variables (from Makefile)

`make run-api` sets:
```bash
ARGUS_SERVER_HTTP_ADDR=0.0.0.0:8080
ARGUS_DATABASE_CLICKHOUSE_DSN=localhost:9000
ARGUS_DATABASE_POSTGRES_DSN="postgresql://argus:argus@localhost:5432/argus?sslmode=disable"
ARGUS_REDIS_ADDR=localhost:6379
```

---

## Port Map

| Port | Service | Protocol | Purpose |
|------|---------|----------|---------|
| 8080 | argus-api | HTTP | REST API, WebSocket, health, metrics |
| 5001 | argus-api | gRPC | Signal streaming, config |
| 9000 | ClickHouse | TCP | Native protocol (high performance) |
| 8123 | ClickHouse | HTTP | HTTP interface (optional) |
| 5432 | PostgreSQL | TCP | Database |
| 6379 | Redis | TCP | Cache |

---

## Production Deployment

### Kubernetes (Helm)

Recommended approach:

1. Build image: `docker build -t argusxdr/argus:v1.0.0 .`
2. Push to registry: `docker push argusxdr/argus:v1.0.0`
3. Create Helm chart: `helm/argus/` (values.yaml, deployment.yaml, service.yaml, etc.)
4. Deploy:
```bash
helm install argus ./helm/argus \
  --set image.tag=v1.0.0 \
  --set clickhouse.host=clickhouse-svc \
  --set postgres.dsn="postgresql://..." \
  --set redis.host=redis-svc
```

### Configuration for Production

Set environment variables:
```bash
ARGUS_LOGGING_LEVEL=info           # Reduce log verbosity
ARGUS_LOGGING_DEV=false            # JSON structured logs
ARGUS_INGEST_QUEUE_CAPACITY=500000 # Larger queue for high throughput
ARGUS_STORAGE_BATCH_SIZE=1000      # Larger batch writes
ARGUS_BASELINE_COMPUTE_INTERVAL=5m # More frequent baselines (optional)
```

Database tuning:
```sql
-- PostgreSQL: index on (app_id, created_at) for alert queries
CREATE INDEX idx_alerts_app_created ON alerts(app_id, created_at DESC);

-- ClickHouse: tune merge behavior for your signal volume
-- ALTER TABLE signals MODIFY SETTING parts_to_throw_insert_when_done = 50;
```

---

## Monitoring

### Health Endpoint

```bash
curl http://localhost:8080/health
{
  "status": "healthy",
  "components": {
    "clickhouse": { "status": "healthy" },
    "postgres": { "status": "healthy" },
    "redis": { "status": "healthy" }
  }
}
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
# HELP ingest_queue_depth Current depth of ingest queue
# TYPE ingest_queue_depth gauge
ingest_queue_depth 42

# HELP storage_writes_total Total signals written to ClickHouse
# TYPE storage_writes_total counter
storage_writes_total 150000
```

Key metrics to monitor:
- `ingest_queue_depth` — growing queue = slow pipeline
- `storage_write_errors_total` — ClickHouse failures
- `detection_alerts_total` — alert rate (should correlate with signal volume)
- `http_requests_duration_seconds{endpoint="/v1/signals"}` — ingest latency p99

---

## Scaling

### Single-Machine

Works up to ~10K signals/sec with:
- 4 CPU cores
- 8GB RAM
- ClickHouse: monthly partitions, ReplacingMergeTree
- PostgreSQL: simple schema, B-tree indexes
- Redis: 512MB max (allkeys-lru eviction)
- 2 ingest pipeline workers (GOMAXPROCS×2)

### Multi-Node (Kubernetes)

1. **ClickHouse**: Separate Kubernetes StatefulSet (or managed service like Yandex Cloud)
2. **PostgreSQL**: Managed database (AWS RDS, Azure Database, etc.)
3. **Redis**: Kubernetes Deployment with PVC, or managed (Redis Cloud, etc.)
4. **Argus API**: Horizontal pod autoscaling based on `ingest_queue_depth` metric

Config StatefulSet:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argus-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: argus
  template:
    metadata:
      labels:
        app: argus
    spec:
      containers:
      - name: argus
        image: argusxdr/argus:v1.0.0
        ports:
        - containerPort: 8080
        - containerPort: 5001
        env:
        - name: ARGUS_DATABASE_CLICKHOUSE_DSN
          value: clickhouse-svc:9000
        - name: ARGUS_DATABASE_POSTGRES_DSN
          valueFrom:
            secretKeyRef:
              name: argus-pg-secret
              key: dsn
```

---

## Related Files

| File | Role |
|------|------|
| `docker-compose.yml` | Three-service dev stack |
| `Dockerfile` | Multi-stage build for production |
| `Makefile` | Orchestration targets |
| `.dockerignore` | Exclude from Docker build |
| `deployments/` | Additional deployment configs |
| `internal/storage/migrations/` | 17 PostgreSQL migrations (auto-applied on startup) |
