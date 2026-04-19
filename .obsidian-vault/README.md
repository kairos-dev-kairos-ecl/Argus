# ArgusXDR Architectural Documentation

Complete bottom-up codebase reference for the ArgusXDR XDR platform.

## How to Use This Vault

1. **Start here**: [[ArgusXDR — Index]] — master navigation
2. **Understand the data**: [[Signal — ArgusSignal Schema]] — the core atom
3. **Trace a signal**: Follow the flow through:
   - [[SDK]] (emission)
   - [[Ingest Pipeline]] (ingestion + processing)
   - [[Storage Layer]] (persistence)
   - [[Detection Engine]] (rule matching)
   - [[Notify & Alerting]] (dispatch)
   - [[Web Dashboard]] (visualization)

4. **Understand infrastructure**:
   - [[Config & Environment]] — how things are configured
   - [[Deployment]] — how to run it
   - [[Auth & RBAC]] — how users are managed
   - [[Baseline Engine]] — how statistical profiles work

5. **Reference**: [[Module Dependencies & Cross-References]] — who calls whom

6. **Deep dives**:
   - [[API Routes]] — all HTTP/gRPC endpoints
   - [[Storage Layer]] — ClickHouse, PostgreSQL, Redis specifics
   - [[Ingest Pipeline]] — the hot path in detail
   - [[Detection Engine]] — rule evaluation (Tier 1/2/3/Kairos)
   - [[Notify & Alerting]] — alert routing and dispatch

---

## Quick Facts

| Aspect | Value |
|--------|-------|
| **Language** | Go 1.24 (core), Python + TypeScript (SDKs) |
| **Core Runtime** | HTTP (chi), gRPC (1.71), Protobuf v2 |
| **Data Model** | ArgusSignal (14 KB .proto schema) |
| **Time-series DB** | ClickHouse 24 (ReplacingMergeTree, 90d TTL) |
| **Relational DB** | PostgreSQL 16 (17 migrations, auth + rules + alerts) |
| **Cache** | Redis 7.2 (512MB, allkeys-lru, all keys TTL'd) |
| **Auth** | RS256 JWT + API keys (SHA256 hash) |
| **Detection** | 3-tier rules (static + baseline + temporal) + Kairos policy engine |
| **Alerting** | Dedup → upsert → incident → routing → 5 notifiers |
| **Dashboard** | React 19 + Vite + TanStack Query + Zustand + ECharts |
| **Signal Throughput** | 10K+ signals/sec (p99 latency < 100ms) |
| **Workers** | Fixed pool (GOMAXPROCS × 2, no goroutine explosion) |
| **Graceful Degradation** | All services optional; API starts even if ClickHouse down |

---

## Document Map

| Document | Covers | Best For |
|----------|--------|----------|
| [[ArgusXDR — Index]] | High-level navigation | Getting oriented |
| [[Signal — ArgusSignal Schema]] | The core data atom (14 KB .proto, 70+ cols) | Understanding what gets captured |
| [[Ingest Pipeline]] | Queue → WorkerPool → Chain (7 processors) | How signals are processed |
| [[Storage Layer]] | ClickHouse + PostgreSQL + Redis | How data persists |
| [[Detection Engine]] | Rules (3 tiers) + Kairos | How threats are detected |
| [[Notify & Alerting]] | AlertRouter → RoutingEngine → Dispatcher → Notifiers | How alerts are sent |
| [[Auth & RBAC]] | JWT + API keys + roles + permissions | How users are managed |
| [[Baseline Engine]] | Background z-score profiling (10 min interval) | How anomalies are scored |
| [[Web Dashboard]] | React 19 + TanStack Query + WebSocket | How operators see signals |
| [[SDK]] | Python + TypeScript clients | How apps emit signals |
| [[API Routes]] | All HTTP + gRPC endpoints | What endpoints exist |
| [[Config & Environment]] | Viper + ARGUS_* env vars | How to configure |
| [[Deployment]] | Docker Compose + Dockerfile + Makefile | How to run it |
| [[Module Dependencies & Cross-References]] | Complete call graph | Who depends on whom |

---

## Signal Journey (Visual)

```
SDK (Python/TypeScript)
  emit_signal(signal)
    ↓ HTTP POST /v1/signals
Ingest Pipeline
  HTTPReceiver
    → AuthValidator (PostgreSQL api_keys)
    → Queue.Enqueue()
  WorkerPool (2-4 goroutines)
    → SchemaValidator
    → Normalizer
    → CorrelationTagger (Redis trace:*)
    → BaselineScorer (Redis/PostgreSQL baseline cache)
    → Enricher (MaxMind GeoIP, Redis geoip:*)
    → DetectionProcessor
         → Tier 1: static matching (layer, category, severity)
         → Tier 2: baseline deviation (z-score ≥ threshold)
         → Tier 3: temporal frequency (Redis det:t3:* ZSET)
         → [optional] Kairos: external policy engine
         → on match: AlertRouter
              ├─ Redis dedup check
              ├─ PostgreSQL upsert alerts
              ├─ Incident correlation (≥3 alerts same trace)
              └─ RoutingEngine + AlertDispatcher
                   → Slack / PagerDuty / Email / Webhook / Syslog
    → BatchWriter
         → ClickHouse signals table (batch 500, flush 2s)
    → SignalBroadcaster
         → WebSocket /v1/signals/stream
         → Web Dashboard live signal table

In parallel:
  Baseline Engine (every 10 min)
    → ClickHouse query (aggregate metrics)
    → ProfileCalculator (mean, stddev)
    → ProfileStore (Redis + PostgreSQL)

  Web Dashboard
    → TanStack Query hooks
    → WebSocket listener
    → Zustand stores (auth, filters)
    → ECharts (visualization)
```

---

## Entry Points

### For Signal Emission
- **Python SDK**: `sdk/client.py` → `ArgusClient.emit_signal()`
- **TypeScript SDK**: `sdk/typescript/src/client.ts` → `ArgusClient.emitSignal()`
- **Test Harness**: `test_harness/qwen_llama_api.py` (E2E example)

### For Understanding the Backend
- **Start**: `cmd/argus/main.go` (cobra + viper init)
- **Then**: `cmd/argus/api.go` (service bootstrap, all handler registration)
- **Hot path**: `internal/ingest/` + `internal/pipeline/` + `internal/storage/`

### For Understanding Detection
- **Rules**: `internal/rules/built-in/` (15 YAML files)
- **Engine**: `internal/detection/engine/engine.go` (Tier 1/2/3 evaluation)
- **Kairos**: `internal/detection/kairos/` (external policy engine integration)

### For Understanding Alerting
- **Router**: `internal/ingest/alert_router.go` (dedup + upsert + incident)
- **Routing**: `internal/notify/router.go` (rule matching + dispatch)
- **Adapters**: `internal/notify/adapters/` (Slack, PagerDuty, Email, Webhook, Syslog)

### For Understanding UI
- **Root**: `web/src/App.tsx` (router setup)
- **Pages**: `web/src/pages/` (20+ pages)
- **Components**: `web/src/components/` (reusable UI)
- **Stores**: `web/src/stores/` (Zustand: auth, signal filters, trace view)
- **Hooks**: `web/src/hooks/` (custom React hooks: useSignals, useWebSocket, etc.)

---

## Key Design Principles

1. **Schema-First**: Everything derives from `ArgusSignal.proto` (14 KB)
2. **Rules-as-Configuration**: Detection rules are YAML data, not compiled code
3. **Graceful Degradation (P3 Pattern)**: All services optional; API starts even if ClickHouse is down
4. **Async Baseline Engine**: Never blocks the ingest hot path
5. **Fixed Worker Pool**: GOMAXPROCS × 2 goroutines (no explosion under load)
6. **Redis TTLs**: All keys are ephemeral; nothing permanent in Redis
7. **Dual Auth**: API keys for signal ingest, JWT for dashboard
8. **No Goroutine Leaks**: WorkerPool.Drain() on shutdown
9. **Non-Fatal Enrichment**: Geo/threat-intel failures skip the signal, don't drop it
10. **Keyset Pagination**: Cursor-based for efficient range queries in ClickHouse

---

## Useful Commands

```bash
# Build
make build

# Run services
make docker-up
make docker-down

# Run API server
make run-api

# Run test harness
make harness-llama

# Validate signals
make validate

# Generate proto stubs
make proto-generate

# Run tests
make test

# Format code
make fmt
```

---

## Common Questions

**Q: How do I emit a signal?**  
A: Use [[SDK#ArgusClient]] (Python or TypeScript). See [[SDK#Usage patterns]].

**Q: How do detection rules work?**  
A: See [[Detection Engine]]. Rules match via Tier 1 (static), Tier 2 (baseline deviation), or Tier 3 (temporal frequency).

**Q: Where are my signals stored?**  
A: [[Storage Layer#ClickHouse]] `signals` table. See [[API Routes#Query Endpoints]] to retrieve them.

**Q: How do I create an alert?**  
A: Detection rules match signals automatically. See [[Notify & Alerting]] for routing and dispatch.

**Q: How do I configure the system?**  
A: [[Config & Environment]] — use `ARGUS_*` environment variables (Viper).

**Q: How do I deploy to production?**  
A: [[Deployment]] — Docker image, Kubernetes Helm, multi-node setup.

**Q: How do I authenticate?**  
A: [[Auth & RBAC]] — API keys for ingest, JWT for dashboard.

---

## Documentation Maintenance

This vault is auto-generated from the ArgusXDR codebase exploration. To update:

1. Regenerate via codebase explorer agent
2. Update cross-references manually for clarity
3. Keep all .md files in `.obsidian-vault/` for Obsidian compatibility

Last updated: 2026-04-16 (approximately)
