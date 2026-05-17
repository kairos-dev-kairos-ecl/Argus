# Argus XDR — Architecture

This document describes how Argus XDR is structured internally: the processing model, storage layout, API surface, auth stack, and how the pieces connect.

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                        SDK / Instrumented Apps                        │
│   Python SDK  ·  TypeScript SDK  ·  gRPC  ·  HTTP  ·  OTLP Bridge   │
└───────────────────────────┬──────────────────────────────────────────┘
                            │ ArgusSignal (proto)
                            ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         Ingest Subsystem                              │
│  gRPC Receiver  ·  HTTP Receiver  ·  OTLP Receiver  →  Queue(100K)  │
└───────────────────────────┬──────────────────────────────────────────┘
                            │ buffered channel
                            ▼
┌──────────────────────────────────────────────────────────────────────┐
│                       Processing Pipeline                             │
│  SchemaValidator → Normalizer → CorrelationTagger → BaselineScorer  │
│                  → Enricher → DetectionProcessor → BatchWriter       │
└───────────┬───────────────────────────────┬──────────────────────────┘
            │ signals                       │ alerts / decisions
            ▼                               ▼
┌─────────────────────┐       ┌────────────────────────────────────────┐
│     ClickHouse      │       │          PostgreSQL                     │
│  signals (primary)  │       │  auth · rules · alerts · incidents      │
│  skip indexes       │       │  baseline_profiles · api_keys           │
│  monthly partitions │       └────────────────────────────────────────┘
└─────────────────────┘
            │                              │
            └──────────────┬───────────────┘
                           │
                    ┌──────▼──────┐
                    │    Redis    │
                    │  ephemeral  │
                    │  trace corr │
                    │  baselines  │
                    └──────┬──────┘
                           │
┌──────────────────────────▼──────────────────────────────────────────┐
│                          Query API                                    │
│  /v1/signals  ·  /api/v1/traces  ·  /api/v1/conversations           │
│  /api/v1/traces/recent  ·  /api/v1/traces/{id}/graph                │
│  /api/v1/conversations/{id}/behaviour                                │
└──────────────┬───────────────────────────────────────────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
┌───▼────┐          ┌─────▼──────────────────────────────┐
│  TUI   │          │         React Dashboard              │
│ (term) │          │  Signals · Traces · Alerts · Rules  │
└────────┘          │  Incidents · Users · Audit · IAM    │
                    └────────────────────────────────────┘
```

---

## Ingest Subsystem

### Receivers

Three receiver types accept signals at the boundary:

**gRPC Receiver** (`internal/ingest/receiver_grpc.go`)
- Implements `IngestService/StreamSignals` — client-streaming RPC
- Uses `Authorization: Bearer <API_KEY>` gRPC metadata
- Best for high-throughput SDK integrations (streaming, binary framing)

**HTTP Receiver** (`internal/ingest/receiver_http.go`)
- `POST /v1/signals` — accepts single signal or array
- Uses `X-Argus-API-Key: <key>` header (not Bearer — this is the ingest auth scheme)
- protojson encoding: field names are camelCase, enum values are proto value names (e.g., `"INTERNAL"` not `"DATA_CLASSIFICATION_INTERNAL"`)

**OTLP Bridge** (`internal/ingest/receiver_otlp.go`)
- `POST /v1/traces`, `POST /v1/metrics` — OTLP JSON or protobuf
- Translates OpenTelemetry spans/metrics into ArgusSignals
- No auth required (OTLP is open for collector compatibility)

### Queue

All receivers write into a single `chan ArgusSignal` with capacity 100,000. This is the backpressure point. If the queue is full, receivers return HTTP 429 / gRPC ResourceExhausted until space opens up.

Monitor: `argus_ingest_queue_depth` Prometheus gauge.

### API Key Authentication

API keys are validated on every ingest request:
1. Key prefix extracted (`argus_sk_` prefix)
2. Hash (SHA-256) compared against `api_keys` table in PostgreSQL
3. Scopes verified (`signals:write` required)
4. Result cached in Redis for 5 minutes (TTL)

Invalid keys return HTTP 401 immediately — they never reach the queue.

---

## Processing Pipeline

The pipeline is a 7-stage serial chain. Each stage receives a signal and passes it to the next. Stages can annotate, enrich, or reject signals; they cannot reorder them.

```
[SchemaValidator] → [Normalizer] → [CorrelationTagger] → [BaselineScorer]
                 → [Enricher] → [DetectionProcessor] → [BatchWriter]
```

### 1. SchemaValidator

Rejects signals missing required fields:
- `signal_id` (non-empty)
- `trace_id` (non-empty)
- `layer` (1–10 or 99 for LDecision)
- `category` (non-empty)
- `timestamp` (non-zero)

Rejected signals are counted in `argus_signals_dropped_total{reason="schema_invalid"}` and never reach storage.

### 2. Normalizer

Canonicalises field values:
- Trims whitespace from string fields
- Normalises `category` to lowercase dotted form
- Ensures `severity` has a valid enum value (defaults to INFO if unset)
- Validates and re-formats timestamps to UTC

### 3. CorrelationTagger

Links signals into traces using Redis sorted sets:
- Key: `trace:<trace_id>` — sorted set of signal timestamps
- Adds the current signal's timestamp/ID to the set
- TTL: 30 seconds (traces are ephemeral — only active windows are tracked in Redis)
- Used downstream by the trace reconstruction layer to build span trees

### 4. BaselineScorer

Computes a z-score deviation for the signal relative to the historical baseline:

```
z = (current_value - baseline_mean) / baseline_stddev
```

Baseline profiles are loaded from Redis (5-minute TTL) and computed by the async `BaselineEngine`. When no baseline exists (insufficient history), `deviation_score` is set to 0.0 and the signal passes through.

**Guard against NaN:** If `baseline_stddev == 0`, the deviation is clamped to 0.0.

### 5. Enricher

Adds derived context that doesn't come from the SDK:
- GeoIP enrichment for `source.remote_ip` fields (database cached in Redis, 24h TTL)
- Cloud provider / region detection from IP ranges
- Internal enrichment tags from rule metadata

### 6. DetectionProcessor

Evaluates configured detection rules against the signal. Rules are YAML data loaded at startup and hot-reloaded. Each rule specifies:
- Layer and category filters
- Threshold conditions on signal fields
- Severity mapping for produced alerts

When a rule fires, it writes an `LDecisionContext` signal to the queue (a separate signal, not a mutation of the original).

### 7. BatchWriter

Buffers signals into ClickHouse batches:
- Batch size: 500 signals (configurable)
- Flush interval: 2 seconds (configurable)
- Uses `clickhouse-go/v2` native protocol (`AsyncInsert` disabled; client-side batching is preferred for predictable latency)
- On flush failure: retries with exponential backoff, logs error, emits `argus_storage_batch_flush_total{status="error"}`

---

## Storage Layer

### ClickHouse — Signal Time-Series

Primary signal store. Table: `signals` using `ReplacingMergeTree`.

Key design decisions:
- **Partition key:** `toYYYYMM(timestamp)` — monthly partitions balance query performance against partition overhead
- **Primary key:** `(app_id, layer, timestamp)` — supports the most common query patterns
- **Skip indexes:** `bloom_filter(0.01)` on `session_id` and `conversation_id` (GRANULARITY 4) — reduces I/O for session/conversation lookups at 1% false positive rate
- **FINAL modifier:** All SELECT queries use `FINAL` to force deduplication (ReplacingMergeTree merges are lazy)
- **DateTime64(9):** Nanosecond timestamps — pass `time.Time` from Go, never `int64` (the Go client treats `int64` as milliseconds)

### PostgreSQL — Configuration and Auth

Stores everything that isn't time-series signal data:

| Table | Purpose |
|-------|---------|
| `users` | User accounts, RBAC roles |
| `sessions` | JWT session tracking, refresh token rotation |
| `api_keys` | API key hashes and scopes |
| `user_backup_codes` | MFA backup codes (hashed) |
| `audit_log` | Immutable audit trail |
| `token_revocations` | Revoked JWT IDs (until expiry) |
| `apps` | Registered applications |
| `detection_rules` | YAML rule configs |
| `alerts` | Produced alerts |
| `incidents` | Grouped incidents (MITRE ATLAS taxonomy) |
| `routing_rules` | Notification routing |
| `baseline_profiles` | Durable baseline computation results |
| `session_baseline_profiles` | Conversation-scoped session baselines |

Migrations: `golang-migrate/migrate/v4` with embedded SQL files in `migrations/`.

### Redis — Ephemeral State

| Key pattern | TTL | Purpose |
|-------------|-----|---------|
| `trace:<id>` | 30s | Active trace correlation sorted set |
| `baseline:<app>:<layer>:<cat>` | 5m | Cached baseline profile |
| `geoip:<ip>` | 24h | Cached GeoIP lookup result |
| `apikey:<hash>` | 5m | Cached API key validation result |
| `rate:<ip>` | 1m | Rate limiter counter |

Redis is non-critical. If Redis is unavailable, the server degrades gracefully: trace correlation is skipped, baseline scores default to 0.0, GeoIP is skipped. Ingest continues.

---

## Behavioural Traceability (Phase 7)

Phase 7 adds three subsystems that operate on top of the stored signal data:

### Trace Reconstruction (`internal/trace/`)

`RunReconstructor` queries ClickHouse for all signals sharing a `trace_id` and builds a typed `Trace` struct with `[]Span` children. `TimelineBuilder` further organises spans into a time-ordered event timeline keyed by `conversation_id`.

### Session Baseline Engine (`internal/baseline/session/`)

Computes per-conversation sequence baselines using 10-minute sliding windows. Produces a `drift_score` measuring how much the current conversation's layer sequence deviates from the historical pattern.

`drift_score = null` means fewer than 10 minutes of data — "not yet computable" rather than "no drift."

### Behaviour Endpoints (`internal/api/behaviour/`)

| Endpoint | Returns |
|----------|---------|
| `GET /api/v1/traces/recent?app_id=X` | Recent run list (trace_id, layers, peak_deviation, duration_ms) |
| `GET /api/v1/traces/{id}/graph` | Nodes + edges + meta for the span tree |
| `GET /api/v1/conversations/{id}/behaviour` | Timeline + session baseline + drift_score |

Route ordering: these routes register **before** the wildcard `{traceID}` route in `api.go` so `traces/recent` resolves as a literal, not a trace ID parameter.

---

## Auth Stack

Full auth reference: `migrations/007_auth.up.sql`, `migrations/010_mfa.up.sql`, `internal/auth/`

### JWT (RS256)

Access tokens expire in 1 hour. Refresh tokens are HttpOnly cookies, rotated on each use. Rotation invalidates the previous refresh token (stored in `sessions` table).

`AuthMiddleware` validates the Bearer token on every protected route. The claims context key is `claimsKey` (defined once in `internal/auth/context.go`).

### RBAC

Roles: `admin`, `analyst`, `viewer`. Permissions are checked at the handler level using `RequireRole(...)` middleware. The `admin` role has every permission; `analyst` has read+write for signals and traces; `viewer` is read-only.

### TOTP / MFA

TOTP is HMAC-SHA1, 6-digit, 30-second window (RFC 6238). Backup codes are one-time use, hashed with bcrypt, stored in `user_backup_codes`. The login flow branches:
1. Credentials check → if MFA disabled, issue token
2. If MFA enabled → return `mfa_required: true` + challenge token
3. Client submits TOTP code → verify + issue full token

### CSRF Protection

Double-submit cookie pattern:
1. `GET /api/v1/auth/csrf-token` → middleware sets `csrf_token` cookie (SameSite=Strict, HttpOnly=false) and returns the same value in `X-CSRF-Token` response header
2. Mutating requests must send both the cookie (automatic) and `X-CSRF-Token` header (explicit)
3. `CSRFMiddleware` constant-time compares them

**Implementation note:** `handleCSRFToken` reads `w.Header().Get("X-CSRF-Token")` — the value the middleware already set on the outgoing response — not `r.Cookie(...)` on the incoming request. Reading the incoming cookie on first visit would fail because the middleware's `Set-Cookie` hasn't reached the client yet.

---

## Dashboard (React + TypeScript)

Entry point: `web/src/main.tsx`

State management:
- `useAuthStore` (Zustand) — JWT, user profile, login/logout
- `useSignalFilters` (Zustand) — filter state persisted across page navigations
- TanStack Query — server state with auto-refetch

WebSocket: `web/src/lib/websocket.ts` connects to `ws://localhost:8080/ws/signals` for live signal feed. The connection is torn down cleanly before close to suppress StrictMode double-connect noise.

Styling: Tailwind v4 with CSS variable tokens defined in `web/src/globals.css`. The design system uses a brutalist dark theme (`#0A0A0B` background, `#FFFFFF` primary text).

---

## TUI (bubbletea)

Entry point: `cmd/argus/tui/behaviour/cmd.go`

The behaviour TUI is a standalone bubbletea application launched via `argus behaviour`. It talks to the API using a JWT Bearer token (passed via `--token` flag or obtained via CSRF login).

Three views controlled by `model.CurrentView ViewState`:
- `ViewRunList` — scrollable list of recent traces with layer badges and deviation
- `ViewRunDetail` — recursive span tree with depth-indented ANSI deviation colouring
- `ViewRunCompare` — side-by-side comparison of two runs (ADDED/REMOVED layers, delta)

`CurrentView` (not `View`) avoids collision with bubbletea's required `View() string` method.

---

## Key Metric Definitions

| Metric | Type | Labels | Meaning |
|--------|------|--------|---------|
| `argus_signals_received_total` | Counter | `receiver` | Signals accepted at the receiver |
| `argus_signals_dropped_total` | Counter | `reason` | Signals rejected (schema, queue full) |
| `argus_ingest_queue_depth` | Gauge | — | Current buffered signal count |
| `argus_storage_batch_flush_total` | Counter | `status` | ClickHouse batch flushes |
| `argus_storage_batch_flush_duration_seconds` | Histogram | — | Flush latency |
| `argus_baseline_profiles_computed_total` | Counter | — | Async baseline computations |
| `argus_http_requests_total` | Counter | `method`, `path`, `status` | API request counts |
