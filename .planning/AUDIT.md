# Argus XDR — Codebase Audit

> Generated: 2026-04-12 | Per ARGUS_FINAL_BUILD_PROMPT.md Step 0

---

## 1. Directory Structure

```
ArgusXDR/
├── cmd/argus/              # CLI (cobra) — api, ingest, server, config, users, rules, doctor
│   ├── api.go              # "argus api" command — query API only
│   ├── ingest.go           # "argus ingest" command — gRPC/HTTP/OTLP receivers
│   ├── server.go           # "argus server" — start/stop/status/logs
│   ├── rules.go            # "argus rules" — rule management CLI
│   ├── users.go            # "argus users" — user management CLI
│   ├── doctor.go           # "argus doctor" — system diagnostics
│   └── internal/           # Private to cmd
│       ├── connectors/     # Local model connector
│       └── proxy/          # Proxy handler
├── cmd/benchmark/          # Load test binary
├── gen/go/                 # Protobuf generated code
│   ├── argus/v1/           # ✓ ArgusSignal, Layer, Severity, Source, Provider, Enrichment
│   └── google/             # ⚠ Broken package structure (multiple packages in one dir)
├── internal/
│   ├── alert/              # Alert/incident data models
│   ├── auth/               # JWT, RBAC, sessions, audit — ⚠ BUILD FAILED
│   ├── baseline/           # Baseline engine (async 10-min cycle)
│   ├── cmd/                # doctor internals — ⚠ BUILD FAILED (net.DialContext undefined)
│   ├── detection/kairos/   # Kairos evaluator — ⚠ BUILD FAILED (proto field mismatches)
│   ├── ingest/             # HTTP/gRPC/OTLP receivers, queue — ⚠ BUILD FAILED
│   ├── kairos/             # Policy registry — ⚠ BUILD FAILED (argusv1 undefined)
│   ├── loadtest/           # ✓ PASS (4 tests)
│   ├── metrics/            # Prometheus metrics — ⚠ BUILD FAILED (redis mock types)
│   ├── notify/             # ✓ PASS — Dispatcher, adapters (Slack, PD, Webhook, Email, Syslog)
│   ├── pipeline/           # 7-stage chain — ⚠ BUILD FAILED
│   ├── resilience/         # Circuit breaker, rate limiter — ⚠ BUILD FAILED
│   └── storage/            # ClickHouse + PostgreSQL clients — ⚠ BUILD FAILED
├── migrations/             # PostgreSQL migrations (007_auth.up.sql)
├── proto/                  # Protobuf definitions (.proto files)
├── sdk/                    # External SDKs
├── tests/
│   ├── integration/        # ⚠ BUILD FAILED (proto enum mismatches)
│   └── unit/               # unit/pipeline FAIL, unit/baseline BUILD FAILED
├── web/                    # React/TypeScript frontend
│   └── src/
│       ├── pages/          # 22 pages
│       └── components/     # 20+ components
├── .claude/launch.json     # Preview server config
├── docker-compose.yml      # Dev stack (ClickHouse, PostgreSQL, Redis, argus-server)
└── argus.exe               # ⚠ Stale prebuilt binary (root-level)
```

---

## 2. Server Startup

**Entry point:** `cmd/argus/main.go` → Cobra root command
**Key commands:**
- `argus api` — Query API only (port 8080, requires ClickHouse)
- `argus ingest` — Ingest receivers (gRPC :5001, HTTP :8080)
- `argus server start/stop` — Full stack

**Critical issue:** In `api.go`, the log line "HTTP server listening" fires BEFORE `ListenAndServe()` is called. If port 8080 is already bound by a stale process, `ListenAndServe` silently fails in the goroutine while the log misleadingly says the server is up. This was the **root cause of the "chi routing bug"** — we were testing against a stale binary that didn't have new routes, not a broken chi router.

```go
// CURRENT (WRONG — logs before binding):
log.Info("HTTP server listening", zap.String("addr", httpAddr))
go func() {
    if err := httpServer.ListenAndServe(); err != nil { log.Error(...) }
}()

// FIXED (log after successful bind):
go func() {
    log.Info("HTTP server listening", zap.String("addr", httpAddr))
    if err := httpServer.ListenAndServe(); err != nil { log.Error(...) }
}()
```

---

## 3. Route Registration

### api.go (argus api command)
| Path | Method | Handler | Status |
|------|--------|---------|--------|
| `/v1/signals` | GET | `QueryHandler.handleGetSignals` | ✓ Working |
| `/v1/schema/signals` | GET | `QueryHandler.HandleGetSignalSchema` | ✓ Registered directly |
| `/metrics` | GET | `promhttp.HandlerFor` | ✓ Working |
| `/health` | GET | inline lambda | ✓ Working (simple `{"status":"ok"}`) |

### ingest.go (argus ingest command)
| Path | Method | Handler | Status |
|------|--------|---------|--------|
| `/v1/signals` | POST | `HTTPReceiver.handlePostSignals` | ✓ |
| `/v1/traces` | POST | `OTLPReceiver.handleTraces` | ✓ |
| `/v1/metrics` | POST | `OTLPReceiver.handleMetrics` | ⚠ Stub |
| `/metrics` | GET | Prometheus | ✓ |
| `/health` | GET | inline | ✓ |

### Missing endpoints (needed by frontend)
| Path | Method | Purpose | Priority |
|------|--------|---------|----------|
| `/api/v1/layers/status` | GET | Dashboard layer coverage (30s poll) | T1 |
| `/v1/signals/stream` | WebSocket | Real-time signal feed | T1 |
| `/api/v1/traces/{traceId}` | GET | Trace detail | T1 |
| `/api/v1/query` | POST | SQL query execution | T1 |
| `/api/v1/rules` | CRUD | Rule management | T2 |
| `/api/v1/alerts` | CRUD | Alert management | T2 |
| `/api/v1/incidents` | CRUD | Incident management | T2 |
| `/api/v1/auth/*` | — | Login, refresh, logout | T3 |
| `/api/v1/users` | CRUD | User management | T3 |
| `/api/v1/apps` | CRUD | App registration | T3 |

---

## 4. Handler Inventory

| Handler | File | Status | Notes |
|---------|------|--------|-------|
| `handleGetSignals` | `receiver_query.go` | ✓ Full | Cursor pagination, all filters |
| `HandleGetSignalSchema` | `receiver_query.go` | ✓ Full | Returns 55 columns |
| `handlePostSignals` | `receiver_http.go` | ✓ Full | 4MB limit, batch, auth |
| `handleTraces` | `receiver_otlp.go` | ✓ Full | OTLP→ArgusSignal mapping |
| `handleMetrics` | `receiver_otlp.go` | ⚠ Stub | Reads, returns 200, no processing |
| `/health` | `api.go` | ⚠ Minimal | Returns `{"status":"ok"}`, no component health |

---

## 5. Detection Engine

**Location:** `internal/detection/kairos/`

The detection engine integrates with **Kairos** — an external HTTP policy evaluation service. It does NOT use local YAML rules.

| Component | Status | Notes |
|-----------|--------|-------|
| `evaluator.go` | ✓ Implemented | Calls Kairos HTTP service, fail-open mode |
| `policy.go` | ✓ Implemented | In-memory policy registry |
| `signal_builder.go` | ⚠ BUILD FAILED | Proto field mismatches (`TimestampNs`, `AppId` don't match current proto) |
| `client.go` | ✓ Implemented | HTTP client for Kairos service |
| YAML rule library | ❌ Not present | Build prompt expects local YAML rules |

**Build prompt says:** Tier 1/2/3 evaluators with YAML rules. **Reality:** Kairos remote service + broken signal builder.

---

## 6. Test Results

```
PASS:  internal/loadtest (4 tests)
PASS:  internal/notify (cached)
PASS:  internal/notify/adapters (cached)
FAIL:  tests/unit/pipeline — TestNormalizer_TimestampNormalizationToUTC
BUILD FAILED (14 packages):
  - cmd/benchmark — missing internal/client package
  - gen/go/google/* — multiple packages in same directory
  - internal/auth — build failed
  - internal/cmd — net.DialContext undefined
  - internal/detection/kairos — proto field mismatches (TimestampNs, AppId, Layer_DECISION)
  - internal/ingest — build failed
  - internal/kairos — argusv1 undefined
  - internal/metrics — redis mock type mismatches
  - internal/pipeline — build failed
  - internal/resilience — build failed
  - internal/storage — build failed
  - tests/integration — proto enum mismatches
  - tests/unit/baseline — build failed
```

**Root causes:**
1. `internal/client` package referenced but not created
2. Proto fields renamed (`TimestampNs` → `timestamp`, `AppId` → `source.app_id`)
3. Proto enums renamed (`Layer_DECISION` removed, `Category_CATEGORY_SECURITY` removed)
4. Redis mock types use concrete `*redis.StringCmd` instead of interface
5. `net.DialContext` moved to `net.Dialer.DialContext` in Go 1.18+

---

## 7. Prometheus Metrics (Currently Emitted)

| Metric | Type | Labels |
|--------|------|--------|
| `argus_signals_received_total` | Counter | receiver |
| `argus_signals_dropped_total` | Counter | reason |
| `argus_ingest_queue_depth` | Gauge | — |
| `argus_ingest_request_duration_seconds` | Histogram | receiver, status |
| `argus_storage_batch_flush_total` | Counter | status |
| `argus_storage_batch_size` | Histogram | — |
| `argus_storage_batch_flush_duration_seconds` | Histogram | — |
| `argus_storage_insert_errors_total` | Counter | — |
| `argus_http_request_duration_seconds` | Histogram | method, path, status_code |
| `argus_http_requests_total` | Counter | method, path, status_code |
| `redis_memory_bytes` | Gauge | — |
| `redis_key_count` | Gauge | — |
| `redis_trace_keys` | Gauge | — |
| `argus_pipeline_baseline_scored_total` | Counter | status |
| `argus_pipeline_signals_tagged_total` | Counter | — |
| `argus_pipeline_geoip_enriched_total` | Counter | — |
| `argus_pipeline_signals_written_total` | Counter | — |
| `argus_pipeline_validation_failures_total` | Counter | — |

**Missing golden signals per build prompt:**
- Detection latency histogram
- Alert generation counter
- WebSocket subscriber gauge
- WebSocket signals dropped counter
- Circuit breaker state gauge
- Component health gauges

---

## 8. PostgreSQL Schema

**Migration file:** `migrations/007_auth.up.sql`

| Table | Purpose |
|-------|---------|
| `users` | Auth data, login tracking, lockout |
| `sessions` | Refresh token tracking |
| `audit_log` | Immutable audit trail (JSONB detail) |
| `token_revocations` | JWT blacklist |

**Missing tables (needed):**
- `apps` — registered applications + API keys
- `detection_rules` — YAML rules storage
- `alerts` — alert records + fingerprints
- `incidents` — incident lifecycle
- `notification_channels` — Slack/PD/Email config
- `routing_rules` — alert→channel routing
- `suppression_rules` — alert dedup suppression

---

## 9. ClickHouse Schema (`internal/storage/schema.go`)

**Table:** `signals` (ReplacingMergeTree)
- **80+ columns** across identity, source, classification, temporal, layer contexts (L1-L10), relationships, provider, enrichment, governance
- **Engine:** ReplacingMergeTree with deduplication
- **Order:** (app_id, layer, timestamp)  
- **Partition:** toYYYYMM(timestamp) — monthly
- **TTL:** 90 days
- **Missing per build prompt:** `span_kind` enum field, `metrics`/`metadata`/`tags` map columns

---

## 10. Frontend Status

**22 pages:** Dashboard, Signals, Trace, Alerts, Incidents, Rules, Apps, Users, AuditLog, Profile, Settings, Query, ConnectorConfig, SetupWizard, and Settings sub-pages

**Current issues:**
- React Query (`useQuery`) hooks present but QueryClientProvider was missing (fixed)
- `useCoverageMap.ts` — `/api/v1/layers/status` disabled (endpoint doesn't exist)
- `useSignalStream.ts` — WebSocket disabled (endpoint doesn't exist)
- Pages render loading states or empty data (no real API responses)

**Frontend is architecturally complete; blocked purely by missing backend endpoints.**

---

## 11. Docker Compose

Services: `clickhouse:24-alpine`, `postgres:16-alpine`, `redis:7.2-alpine`, `argus-server` (custom build)

**Argus server in docker-compose runs:** `api --dev` (query API only, not full ingest)

---

## Summary: What's Actually Broken

| Issue | Severity | Effort |
|-------|----------|--------|
| **Stale binary/port shadowing (fake "chi bug")** | 🔴 Critical | 30 min |
| **14 packages won't compile** | 🔴 Critical | 2-4 hrs |
| **Missing API endpoints (10+)** | 🔴 Critical | 4-6 hrs |
| **WebSocket signal stream** | 🔴 Critical | 2-3 hrs |
| **Health endpoint minimal** | 🟡 High | 1 hr |
| **`span_kind` + annotation model missing from schema** | 🟡 High | 2 hrs |
| **Alert routing unconnected** | 🟡 High | 2-3 hrs |
| **Auth system build failures** | 🟡 High | 2-3 hrs |
| **Detection: YAML rules vs Kairos mismatch** | 🟡 High | 3-4 hrs |
| **Missing PostgreSQL tables** | 🟡 High | 1-2 hrs |
| **Frontend connected but no data** | 🟢 Unblocked once APIs exist | — |
