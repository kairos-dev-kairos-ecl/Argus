<div align="center">
  <img src="assets/logo.png" alt="Argus XDR Logo" width="180">

  # Argus XDR

  **Open-Source Extended Detection & Response for LLM-Integrated Systems**

  [![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
  [![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?logo=go)](https://golang.org/)
  [![TypeScript](https://img.shields.io/badge/typescript-5.4+-3178C6?logo=typescript)](https://www.typescriptlang.org/)
  [![Python](https://img.shields.io/badge/python-3.10+-3776AB?logo=python)](https://www.python.org/)

</div>

---

Argus is a production-grade XDR platform purpose-built for LLM-integrated systems. It provides full-stack signal coverage across a 10-layer LLM system taxonomy — from GPU hardware (L1) through orchestration (L9) and application behaviour (L10) — with correlated threat detection, behavioural traceability, investigation workflows, and response orchestration.

**Core value:** Every signal from every layer of an LLM system is captured, normalised into a unified schema, correlated across traces, and surfaced to operators with full detection and investigation capability — so threats, anomalies, and behavioural drift are never invisible.

---

## What Argus Covers

```
┌──────────────────────────────────────────────────────────────┐
│  LDecision  Policy enforcement · alerts · incident creation  │
├──────────────────────────────────────────────────────────────┤
│  L10  Application       User sessions, feature flags, UX     │
│  L9   Orchestration     Agent coordination, multi-model      │
│  L8   Data Access       RAG, vector search, retrieval        │
│  L7   Tool Use          Function calls, plugin execution     │
│  L6   Integration       External API calls                   │
│  L5   Output Decoding   Sampling, filtering, streaming       │
│  L4   Inference         Forward pass, token counts, latency  │
│  L3   Model Loading     Weights, quantisation, adapters      │
│  L2   Runtime           ML framework (PyTorch, JAX, etc.)    │
│  L1   Hardware          GPU/CPU/memory at the system level   │
└──────────────────────────────────────────────────────────────┘
```

Every signal across all 11 layers shares the same `ArgusSignal` protobuf envelope — type-safe, layer-specific context fields, unified correlation by `trace_id` and `conversation_id`.

---

## Quick Start

### Docker Compose (full stack)

```bash
git clone https://github.com/argusxdr/argus.git
cd argus

# Start Argus + ClickHouse + PostgreSQL + Redis
docker compose up -d

# Verify all services are healthy
curl http://localhost:8080/health
# → {"status":"healthy","clickhouse":{"status":"ok"},"postgres":{"status":"ok"},"redis":{"status":"ok"}}
```

Open `http://localhost:8080` for the first-run setup wizard. It creates the admin account and your first API key.

### Send your first signal

```bash
curl -X POST http://localhost:8080/v1/signals \
  -H "X-Argus-API-Key: argus_sk_YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "signal_id": "sig-001",
    "trace_id":  "trace-test-001",
    "layer":     4,
    "category":  "inference.latency.normal",
    "severity":  "INFO",
    "timestamp": "2026-05-18T10:00:00Z",
    "source":    {"app_id": "my-app"}
  }'
# → {"accepted":1,"rejected":0}
```

### Python SDK

```python
from sdk.client import ArgusClient
from sdk.signal_builder import SignalBuilder
import asyncio

async def main():
    client = ArgusClient(base_url="http://localhost:8080", api_key="argus_sk_...")
    signal = (
        SignalBuilder()
        .layer(4).category("inference.latency.normal").severity("INFO")
        .trace_id("trace-001").source(app_id="my-app")
        .l4_context(model_id="gpt-4o", latency_ms=320, prompt_tokens=128, completion_tokens=64)
        .build()
    )
    print(await client.ingest([signal]))  # {"accepted": 1, "rejected": 0}

asyncio.run(main())
```

---

## Architecture

```
SDK / Apps (Python · TypeScript · gRPC · HTTP · OTLP)
         ↓
  Ingest Receivers  →  Queue (100K)
         ↓
  Processing Pipeline (7 stages):
    SchemaValidator → Normalizer → CorrelationTagger
    → BaselineScorer → Enricher → DetectionProcessor → BatchWriter
         ↓                              ↓
    ClickHouse                    PostgreSQL
  (signals, traces)          (auth, rules, alerts,
   monthly partitions          incidents, baselines)
         ↓
      Redis (trace correlation, baseline cache, rate limiting)
         ↓
    Query API  (/v1/signals · /api/v1/traces · /api/v1/conversations)
         ↓                    ↓
   bubbletea TUI        React Dashboard
```

**Storage:**
- **ClickHouse** — time-series signal store, `ReplacingMergeTree`, monthly partitions, bloom_filter skip indexes on `session_id`/`conversation_id`
- **PostgreSQL** — users, sessions, API keys, detection rules, alerts, incidents, baseline profiles
- **Redis** — ephemeral trace correlation (30s TTL), baseline cache (5m TTL), rate limiting

**Processing pipeline** runs at GOMAXPROCS×2 goroutines. Baseline scoring uses an async 10-minute computation cycle — it never blocks the ingest hot path.

---

## Features

### Signal Ingestion
- gRPC streaming receiver (`IngestService/StreamSignals`)
- HTTP receiver (`POST /v1/signals`) with protojson encoding
- OTLP bridge (`POST /v1/traces`, `POST /v1/metrics`) for OpenTelemetry compatibility
- 100K-signal internal buffer with backpressure to receivers
- 10K+ signals/sec sustained throughput

### Detection & Baselines
- YAML-defined detection rules — data, not code
- Async baseline engine: per `(app_id, layer, category)` z-score profiles, computed from ≥100 samples
- Deviation scoring on every signal relative to its historical baseline
- GeoIP enrichment for source IP fields

### Behavioural Traceability
- Trace reconstruction: all signals sharing a `trace_id` assembled into a typed span tree
- Conversation timeline: session-scoped event ordering across multiple traces
- Session baseline engine: drift scoring comparing current conversation sequence to historical pattern
- API endpoints: `GET /api/v1/traces/recent`, `GET /api/v1/traces/{id}/graph`, `GET /api/v1/conversations/{id}/behaviour`

### Authentication & Security
- JWT (RS256) with 1-hour access tokens and rotating HttpOnly refresh cookies
- RBAC: `admin`, `analyst`, `viewer` roles with permission-based enforcement
- TOTP/MFA with RFC 6238 implementation and one-time backup codes
- API keys with scope control (`signals:write`) and Redis-cached validation
- CSRF double-submit cookie protection on all auth mutation endpoints
- HIBP password breach check on account creation (fail-open on network error)

### Operator Interfaces
- **React Dashboard** (`web/`) — 22 pages: Signals, Traces, Alerts, Rules, Incidents (MITRE ATLAS), Users, Audit, API Keys, IAM, Settings
- **bubbletea TUI** (`argus behaviour`) — run list, span tree detail, side-by-side run comparison, layer badges, deviation colouring

### Notifications
- Dispatcher with adapters: Slack, PagerDuty, Email, Webhook, Syslog

---

## Project Layout

```
.
├── cmd/argus/               CLI entry points (Cobra)
│   ├── main.go              Root command + dispatch
│   ├── server.go            `argus server` — full stack
│   ├── api.go               HTTP router + route registration
│   └── tui/                 bubbletea TUI (selector + behaviour screens)
├── internal/
│   ├── ingest/              Receivers, queue, batch writer, auth
│   ├── pipeline/            7-stage processing chain
│   ├── storage/             ClickHouse, PostgreSQL, Redis clients
│   ├── auth/                JWT, CSRF, RBAC, TOTP, session management
│   ├── baseline/            Async baseline engine + session drift scoring
│   ├── trace/               RunReconstructor, TimelineBuilder
│   ├── api/                 HTTP handlers (behaviour, query, auth, etc.)
│   ├── detection/           Rule evaluator (Kairos sidecar integration)
│   ├── notify/              Notification dispatcher + adapters
│   └── rules/               Built-in YAML detection rules
├── proto/argus/v1/          Protobuf schema (signal.proto, service.proto)
├── gen/go/argus/v1/         Generated Go stubs
├── sdk/
│   ├── client.py            Python ArgusClient (httpx, async)
│   ├── signal_builder.py    Fluent SignalBuilder
│   └── typescript/          TypeScript client + builder
├── web/                     React + TypeScript dashboard (Vite, shadcn/ui)
├── migrations/              PostgreSQL migration SQL (golang-migrate)
├── docs/                    Documentation
│   ├── getting-started.md
│   ├── architecture.md
│   ├── signal-taxonomy.md
│   ├── configuration.md
│   └── contributing.md
├── docker-compose.yml       Full dev stack
├── Dockerfile               Multi-stage Go build
└── Makefile                 build, test, lint, proto, docker targets
```

---

## Configuration

Argus reads `argus.yaml` in the current directory. All keys can be overridden with `ARGUS_`-prefixed environment variables.

```yaml
server:
  http_addr: "localhost:8080"
  grpc_addr: "localhost:5001"

storage:
  clickhouse:
    addr:     "localhost:9000"
    database: "default"
  postgres:
    dsn: "postgres://argus:argus@localhost:5432/argus"
  redis:
    addr: "localhost:6379"

ingest:
  queue_capacity: 100000
  batch_size:     500
  flush_interval: "2s"

logging:
  level:  "info"
  format: "json"
```

Full reference: [`docs/configuration.md`](docs/configuration.md)

---

## API Reference

### Ingest

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/v1/signals` | `X-Argus-API-Key` | Ingest one or more signals |
| POST | `/v1/traces` | None | OTLP trace bridge |
| POST | `/v1/metrics` | None | OTLP metrics bridge |
| STREAM | gRPC `IngestService/StreamSignals` | Bearer metadata | High-throughput gRPC stream |

### Query

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/v1/signals` | None | Query signals (app_id, layer, severity, time range, cursor) |
| GET | `/api/v1/traces/recent` | JWT | Recent run list for an app |
| GET | `/api/v1/traces/{id}/graph` | JWT | Span tree (nodes + edges + meta) |
| GET | `/api/v1/conversations/{id}/behaviour` | JWT | Timeline + session drift score |

### Auth

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/auth/csrf-token` | None | Fetch CSRF token (sets cookie) |
| POST | `/api/v1/auth/login` | CSRF | Login → access_token + refresh cookie |
| POST | `/api/v1/auth/refresh` | Refresh cookie | Rotate tokens |
| POST | `/api/v1/auth/logout` | JWT | Revoke session |

Full auth protocol: see `CLAUDE.md` → Auth Client Protocol section.

---

## Building from Source

```bash
# Go binary
go build -o ./argus ./cmd/argus
./argus --help

# Frontend
cd web && npm install && npm run build

# Regenerate protobuf stubs (requires buf CLI)
buf generate

# Run tests
go test ./...
go test -race ./...

# Lint
golangci-lint run
```

---

## Performance

| Metric | Target |
|--------|--------|
| Ingest throughput | 10K+ signals/sec sustained |
| Ingest latency p99 | <100ms (baseline scoring is async) |
| Detection latency | <100ms from ingestion to alert |
| Query latency | <1s for cursor-paginated 100K result set |
| SDK overhead | <5ms p99 per signal |
| Queue capacity | 100K buffered signals |

---

## Documentation

| Doc | Contents |
|-----|---------|
| [`docs/getting-started.md`](docs/getting-started.md) | Step-by-step first signal walkthrough |
| [`docs/architecture.md`](docs/architecture.md) | Processing pipeline, storage, auth, TUI internals |
| [`docs/signal-taxonomy.md`](docs/signal-taxonomy.md) | All 11 layers, fields, categories, severity |
| [`docs/configuration.md`](docs/configuration.md) | Full config reference, environment variables |
| [`docs/contributing.md`](docs/contributing.md) | Code conventions, PR workflow, security policy |
| [`docs/worktree-workflow.md`](docs/worktree-workflow.md) | Branch model and development worktree structure |

---

## Contributing

See [`docs/contributing.md`](docs/contributing.md). In short:
- Branch from `dev`, PR targets `dev`
- Tests required for all new behaviour
- Proto changes are breaking — discuss before opening a PR
- Security issues: email `security@argusxdr.io`, not a public issue

---

## Support

- **GitHub Issues:** https://github.com/kairos-dev-kairos-ecl/Argus/issues
- **Discussions:** https://github.com/kairos-dev-kairos-ecl/Argus/discussions
- **Documentation:** https://github.com/kairos-dev-kairos-ecl/Argus/tree/main/docs

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
