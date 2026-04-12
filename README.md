<div align="center">
  <img src="assets/logo.png" alt="Argus XDR Logo" width="180">
  
  # Argus XDR
  
  **Production-Grade Extended Detection & Response Platform for LLM Systems**
  
  [![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
  [![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](https://golang.org/)
  [![TypeScript](https://img.shields.io/badge/typescript-5.4+-3178C6?logo=typescript)](https://www.typescriptlang.org/)
  [![Python](https://img.shields.io/badge/python-3.10+-3776AB?logo=python)](https://www.python.org/)
  
</div>

---

Argus is an open-source XDR platform purpose-built for LLM-integrated systems. It provides full-stack signal coverage across a 10-layer LLM system taxonomy (L1 Hardware through L10 Application), correlated threat detection, investigation workflows, and response orchestration.

**Core Value:** Every signal from every layer of an LLM system is captured, normalized into a unified schema, correlated across traces, and surfaced to operators with full detection and investigation capability — so threats, anomalies, and behavioral drift are never invisible.

## Quick Start

### Local Development (Zero Dependencies)

```bash
# Build the binary
go build -o ./argus ./cmd/argus

# Start in dev mode (no external services required)
./argus server --dev

# In another terminal, test the health endpoint
curl http://localhost:8080/health
# → {"status":"ok"}

# Test metrics endpoint
curl http://localhost:8080/metrics | head -20
# → prometheus format text
```

### Docker Compose (Full Stack)

```bash
# Start all services (Argus, ClickHouse, PostgreSQL, Redis)
docker compose up -d

# Wait for services to be healthy (~30 seconds)
docker compose ps
# All services should show "healthy"

# Test the health endpoint
curl http://localhost:8080/health
# → {"status":"ok"}

# Test metrics
curl http://localhost:8080/metrics | head -5

# Send a test signal via HTTP
curl -X POST http://localhost:8080/v1/signals \
  -H "Authorization: Bearer dev-test-key" \
  -H "Content-Type: application/json" \
  -d '{
    "signal_id": "test-01",
    "trace_id": "trace-abc",
    "layer": "L5_OUTPUT_DECODING",
    "category": "output.completion",
    "severity": "INFO",
    "source": { "app_id": "test-app" },
    "timestamp": "2026-04-07T12:00:00Z"
  }'
# → {"accepted":1,"rejected":0}

# Wait for signal to flush
sleep 3

# Query it back
curl "http://localhost:8080/v1/signals?app_id=test-app"

# Shutdown
docker compose down -v
```

### Kubernetes (Helm)

```bash
# Add Argus Helm repository (when available)
# helm repo add argus https://charts.argusxdr.io
# helm repo update

# Install the chart
helm install argus ./deployments/helm \
  --namespace argus \
  --create-namespace \
  --values ./deployments/helm/values.yaml

# Verify deployment
kubectl get pods -n argus
kubectl get svc -n argus

# Forward ports for testing
kubectl port-forward -n argus svc/argus-api 8080:8080
kubectl port-forward -n argus svc/argus-grpc 5001:5001

# View logs
kubectl logs -n argus -l app.kubernetes.io/name=argus -f

# Upgrade
helm upgrade argus ./deployments/helm -n argus

# Uninstall
helm uninstall argus -n argus
```

## CLI Commands

### `argus server` — All-in-One Server (PLATFORM-01)

Start all subsystems (gRPC receiver, HTTP receiver, OTLP bridge, query API) in a single process.

```bash
argus server [FLAGS]

FLAGS:
  --dev                         Use embedded storage (zero external deps)
  --config string               Path to config YAML (default: argus.yaml)
  --grpc-addr string           gRPC listen address (default: localhost:5001)
  --http-addr string           HTTP listen address (default: localhost:8080)
  --postgres-dsn string         PostgreSQL connection string
  --clickhouse-dsn string       ClickHouse connection string
  --queue-capacity int          Ingest queue capacity (default: 100000)
  --batch-size int              ClickHouse batch writer size (default: 500)
  --batch-interval duration     Batch flush interval (default: 2s)
  --dev                         Enable dev mode
```

**Production mode:** Requires PostgreSQL, ClickHouse, and Redis running.
**Dev mode (`--dev`):** Zero-dependency mode using in-memory storage and hardcoded auth key `dev-key-argus`.

### `argus ingest` — Ingest Subsystem Only (PLATFORM-02)

Start only the ingest receivers (gRPC, HTTP, OTLP, queue) without the query API.

```bash
argus ingest [FLAGS]
# Same flags as server (except query API endpoints are not registered)
```

### `argus api` — Query API Subsystem Only (PLATFORM-02)

Start only the query API (GET /v1/signals) without ingest receivers.

```bash
argus api [FLAGS]

FLAGS:
  --http-addr string           HTTP listen address (default: localhost:8080)
  --clickhouse-dsn string       ClickHouse connection string
```

## Endpoints

### Ingest

- **POST /v1/signals** (HTTP) — Ingest signals via HTTP
  - Requires: `Authorization: Bearer <API_KEY>` header
  - Body: Single `ArgusSignal` object or array of objects
  - Response: `{"accepted": N, "rejected": M}`

- **POST /v1/traces** (HTTP) — Ingest OpenTelemetry traces
  - Body: OTLP ExportTraceServiceRequest (protobuf or JSON)
  - No auth required (OTLP is open for compatibility)

- **POST /v1/metrics** (HTTP) — Ingest OpenTelemetry metrics
  - Body: OTLP ExportMetricsServiceRequest (protobuf or JSON)
  - No auth required

- **argus.v1.IngestService/StreamSignals** (gRPC) — Stream signals via gRPC
  - Requires: `Authorization: Bearer <API_KEY>` metadata
  - Request: Stream of `ArgusSignal` messages
  - Response: `BatchResult { accepted, rejected }`

### Query

- **GET /v1/signals** — Query signals with cursor pagination (STORE-06)
  - Query params:
    - `app_id` (required): Filter by application ID
    - `layer` (optional): Filter by layer (1-10)
    - `category` (optional): Filter by category
    - `severity` (optional): Minimum severity (1-5)
    - `start` (optional): RFC3339 timestamp lower bound
    - `end` (optional): RFC3339 timestamp upper bound
    - `cursor` (optional): Pagination cursor from previous response
    - `limit` (optional): Results per page (default: 100, max: 1000)
  - Response: `{"signals": [...], "next_cursor": "...", "total_hint": N}`

### Observability

- **GET /health** — Health check endpoint
  - Response: `{"status":"ok"}`

- **GET /metrics** — Prometheus metrics endpoint (PLATFORM-05)
  - Response: Prometheus text format
  - Key metrics:
    - `argus_signals_received_total{receiver="grpc"|"http"|"otlp"}`
    - `argus_signals_dropped_total{reason="..."`
    - `argus_ingest_queue_depth` (current queue size)
    - `argus_storage_batch_flush_total` (flush count)

## Configuration

### Config File (argus.yaml)

```yaml
server:
  grpc_port: 5001
  http_port: 8080

storage:
  clickhouse:
    addr: "localhost:9000"
    database: "default"
    username: "default"
    password: ""
    max_open_conns: 10
    dial_timeout: 5s

  postgres:
    dsn: "postgres://argus:argus@localhost:5432/argus?sslmode=disable"
    max_conns: 20
    min_conns: 5

  redis:
    addr: "localhost:6379"
    password: ""
    db: 0

ingest:
  queue_capacity: 100000
  batch_size: 500
  flush_interval: 2s
  max_grpc_message_bytes: 10485760  # 10MB
  max_http_body_bytes: 4194304      # 4MB
  auth_cache_ttl: 5m

logging:
  level: "info"   # debug, info, warn, error
  format: "json"  # json, console
```

### Environment Variables

All config values can be overridden via environment variables with prefix `ARGUS_`:

```bash
export ARGUS_STORAGE_CLICKHOUSE_ADDR=clickhouse:9000
export ARGUS_STORAGE_POSTGRES_DSN=postgres://...
export ARGUS_STORAGE_REDIS_ADDR=redis:6379
export ARGUS_SERVER_GRPC_ADDR=0.0.0.0:5001
export ARGUS_SERVER_HTTP_ADDR=0.0.0.0:8080
export ARGUS_INGEST_QUEUE_CAPACITY=100000
export ARGUS_LOGGING_LEVEL=debug
```

## Architecture

### Subsystems

1. **Ingest** — Receives signals from SDKs, OpenTelemetry, and webhooks
   - gRPC receiver (StreamSignals RPC)
   - HTTP receiver (POST /v1/signals)
   - OTLP bridge (POST /v1/traces, /v1/metrics)
   - Queue (100K buffered channel)
   - Batch writer (500 rows or 2s flush)

2. **Storage** — Persists signals and metadata
   - ClickHouse: Signal time-series (ReplacingMergeTree, monthly partitions)
   - PostgreSQL: Config, auth, rules, incidents
   - Redis: Ephemeral trace correlation, deduplication, rate limiting

3. **Query API** — Exposes signals for investigation and dashboard
   - GET /v1/signals with cursor pagination (STORE-06)
   - Filters: app_id, layer, category, severity, timestamp range
   - Direct ClickHouse queries with FINAL modifier (dedup)

4. **Metrics** — Prometheus-compatible observability
   - Signals received/dropped by receiver type
   - Queue depth gauge
   - Batch flush counts and latencies

## Testing

### Smoke Test (docker-compose)

```bash
# Run the smoke test script
bash test/smoke_test.sh
```

The script:
1. Starts docker compose
2. Sends 10 signals via gRPC
3. Sends 10 signals via HTTP
4. Waits for batch flush
5. Queries all 20 signals back
6. Verifies metrics
7. Shuts down

### Unit Tests

```bash
go test ./... -v
go test -race ./...
```

### Integration Tests

```bash
# Start docker compose
docker compose up -d

# Run tests against real services
go test -tags integration ./test

# Cleanup
docker compose down -v
```

## Performance Targets

- **Ingest throughput:** 10K+ signals/sec sustained
- **Detection latency:** <100ms (from ingestion to alert)
- **Query latency:** <1s for 100K signal result set (cursor paginated)
- **Queue capacity:** 100K buffered signals
- **Batch size:** 500 signals (flush every 2 seconds or sooner)
- **SDK overhead:** <5ms p99 on instrumented applications (per signal)

## Deployment

### Docker Compose (Development/Testing)

```bash
docker compose up -d
docker compose ps
docker compose logs -f argus-server
docker compose down -v
```

### Helm (Kubernetes/Production)

```bash
helm install argus ./deployments/helm -n argus --create-namespace

# Scale ingest replicas
kubectl scale deployment/argus-ingest -n argus --replicas=5

# View HPA status
kubectl get hpa -n argus
```

### systemd (Linux VM/Bare Metal)

```bash
# Create service file
sudo tee /etc/systemd/system/argus.service > /dev/null <<EOF
[Unit]
Description=Argus XDR Server
After=network.target

[Service]
Type=simple
User=argus
ExecStart=/usr/local/bin/argus server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable argus
sudo systemctl start argus
sudo systemctl status argus
```

## Troubleshooting

### Connection Errors

```bash
# ClickHouse unreachable
./argus server 2>&1 | grep clickhouse
# Expected: "connecting to ClickHouse", "clickhouse connected"

# PostgreSQL unreachable
./argus server 2>&1 | grep postgres
# Expected: "connecting to PostgreSQL", "postgresql connected"

# Redis unreachable (non-fatal, degrades trace correlation)
./argus server 2>&1 | grep redis
```

### Queue Backlog

Monitor `argus_ingest_queue_depth` metric:

```bash
curl http://localhost:8080/metrics | grep argus_ingest_queue_depth
```

If queue is growing:
- Scale ingest pods (increase replicas or batch size)
- Check ClickHouse write performance
- Check network latency to ClickHouse

### High Dropped Signals

Monitor `argus_signals_dropped_total{reason="queue_full"}`:

```bash
curl http://localhost:8080/metrics | grep argus_signals_dropped_total
```

Solutions:
- Increase `--queue-capacity` flag
- Scale ingest replicas (Kubernetes)
- Increase batch size or flush interval

## Development

### Building the Binary

```bash
go mod tidy
go build -o ./argus ./cmd/argus
./argus --help
```

### Code Structure

```
.
├── cmd/argus/              # CLI entry points
│   ├── main.go            # Cobra root command
│   ├── server.go          # `argus server` subcommand (all subsystems)
│   ├── ingest.go          # `argus ingest` subcommand (receivers only)
│   └── api.go             # `argus api` subcommand (query API only)
├── gen/go/argus/v1/       # Protobuf generated Go code
│   ├── signal.pb.go       # ArgusSignal message definition
│   ├── service.pb.go      # RPC service definitions
│   └── service_grpc.pb.go # gRPC server/client stubs
├── internal/
│   ├── ingest/            # Signal ingestion subsystem
│   │   ├── queue.go       # Buffered signal queue
│   │   ├── auth.go        # API key validation
│   │   ├── receiver_grpc.go      # gRPC ingest receiver
│   │   ├── receiver_http.go      # HTTP ingest receiver
│   │   ├── receiver_otlp.go      # OTLP bridge receiver
│   │   └── receiver_query.go     # Query API handler
│   ├── storage/           # Data persistence
│   │   ├── clickhouse.go  # ClickHouse client + batch writer
│   │   ├── postgres.go    # PostgreSQL client + migrations
│   │   ├── redis.go       # Redis client (trace correlation)
│   │   ├── schema.go      # ClickHouse DDL
│   │   └── migrations/    # PostgreSQL migration SQL files
│   └── metrics/           # Prometheus metrics
│       └── metrics.go     # Metric definitions (Ingest, Storage, HTTP)
├── proto/                 # Protocol Buffer definitions
│   └── argus/v1/
│       ├── signal.proto         # Core ArgusSignal message
│       ├── service.proto        # gRPC service definitions
│       └── categories.proto     # Signal category taxonomy
├── deployments/
│   └── helm/              # Kubernetes Helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
├── test/                  # Tests and smoke test script
│   ├── smoke_test.sh
│   └── (unit/integration tests)
├── Dockerfile             # Multi-stage Docker build
├── docker-compose.yml     # Dev/test stack (Argus + backends)
└── README.md              # This file
```

### Running Tests

```bash
# Unit tests
go test ./internal/... -v

# Integration tests (requires docker compose up)
go test -tags integration ./test -v

# Race detector
go test -race ./...

# Coverage
go test ./... -cover
```

### Debugging

```bash
# Dev mode with verbose logging
./argus server --dev 2>&1 | grep -E "starting|listening|queue|shutdown"

# Watch metrics
watch -n 1 'curl -s http://localhost:8080/metrics | grep "argus_"'

# Inspect ClickHouse data
docker compose exec clickhouse clickhouse-client --query "SELECT COUNT(*) FROM signals;"

# Inspect PostgreSQL
docker compose exec postgres psql -U argus -d argus -c "SELECT COUNT(*) FROM api_keys;"
```

## Contributing

Argus XDR is open source and welcomes contributions. See CONTRIBUTING.md for guidelines.

## License

Apache License 2.0 — See LICENSE file for details.

## Support

- GitHub Issues: https://github.com/argusxdr/argus/issues
- Discussions: https://github.com/argusxdr/argus/discussions
- Documentation: https://docs.argusxdr.io
