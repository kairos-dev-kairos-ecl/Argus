<!-- GSD:project-start source:PROJECT.md -->
## Project

**Argus XDR**

Argus is an open-source, production-grade Extended Detection & Response (XDR) platform purpose-built for LLM-integrated systems. It provides full-stack signal coverage across a 10-layer LLM system taxonomy (L1 Hardware through L10 Application), correlated threat detection, investigation workflows, and response orchestration. It treats LLM systems the way modern XDR treats enterprise infrastructure — with observable, auditable, queryable signal pipelines.

**Core Value:** Every signal from every layer of an LLM system is captured, normalized into a unified schema, correlated across traces, and surfaced to operators with full detection and investigation capability — so that threats, anomalies, and behavioral drift are never invisible.

### Constraints

- **Tech Stack**: Go core, ClickHouse signals, PostgreSQL config, Redis ephemeral, React+TS dashboard, protobuf/gRPC wire protocol — locked per spec
- **Charting**: Apache ECharts (not Plotly) — handles 100K+ data points, canvas rendering
- **Schema First**: Everything depends on the protobuf schema — it is the contract
- **Detection as Configuration**: Rules are YAML data, not compiled code
- **Performance**: Target 10K+ signals/sec sustained ingestion, sub-100ms detection latency
- **SDK overhead**: <5ms p99 overhead on instrumented applications
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Core Runtime: Go
| Concern | Library | Version | Rationale |
|---------|---------|---------|-----------|
| gRPC server/client | `google.golang.org/grpc` | v1.63+ | Official Go gRPC, battle-tested |
| Protobuf codegen | `google.golang.org/protobuf` | v1.34+ | New API (not deprecated `github.com/golang/protobuf`) |
| HTTP router | `github.com/go-chi/chi/v5` | v5.0 | Lightweight, stdlib-compatible, middleware-friendly |
| Config | `github.com/spf13/viper` | v1.18+ | YAML/env/flag unification, hot reload support |
| CLI | `github.com/spf13/cobra` | v1.8+ | Pairs with Viper, used by kubectl/helm |
| Logging | `go.uber.org/zap` | v1.27+ | Structured, zero-alloc, production-grade |
| Metrics | `github.com/prometheus/client_golang` | v1.19+ | Prometheus native, widely adopted |
| Testing | stdlib `testing` + `github.com/stretchr/testify` | v1.9+ | No mock magic, assertion sugar only |
| ULID | `github.com/oklog/ulid/v2` | v2.1+ | Time-sortable unique IDs for signal_id |
| UUID | `github.com/google/uuid` | v1.6+ | For trace_id compatibility |
| Validation | `github.com/go-playground/validator/v10` | v10.20+ | Struct tag validation |
- `gorilla/mux` — unmaintained
- `gin` — fine but chi is lighter for this use case
- `github.com/golang/protobuf` — deprecated, use `google.golang.org/protobuf`
## Protobuf Tooling
| Tool | Purpose |
|------|---------|
| `protoc` v25+ | Schema compiler |
| `protoc-gen-go` v1.34+ | Go server stubs |
| `protoc-gen-go-grpc` v1.3+ | Go gRPC stubs |
| `protoc-gen-python` (bundled) | Python client stubs |
| `ts-proto` v1.176+ | TypeScript client stubs (generates idiomatic TS, not JS wrappers) |
| `buf` CLI v1.30+ | Schema linting, breaking change detection, registry |
## ClickHouse
| Concern | Choice | Notes |
|---------|--------|-------|
| Go client | `github.com/ClickHouse/clickhouse-go/v2` | v2.24+ — native protocol, connection pooling |
| Version target | ClickHouse 24.x LTS | Stable, materialized views, JSON type (experimental) |
| Table engine | `MergeTree` family | `ReplacingMergeTree` for dedup, `SummingMergeTree` for pre-agg |
| Partition key | `toYYYYMM(timestamp)` | Monthly partitions, balance query perf vs partition overhead |
| Insert method | Async batch via `AsyncInsert` or client-side batching | Never single-row inserts |
- HTTP interface for bulk inserts — 5-10x slower than native protocol
- `database/sql` interface — misses ClickHouse-specific features
## PostgreSQL
| Concern | Choice |
|---------|--------|
| Go driver | `github.com/jackc/pgx/v5` | v5.5+ — pgx is faster than `database/sql`+`lib/pq` |
| Migrations | `github.com/golang-migrate/migrate/v4` | Embedded SQL files, CLI + programmatic |
| Version | PostgreSQL 16 | JSONB improvements, logical replication |
## Redis
| Concern | Choice |
|---------|--------|
| Go client | `github.com/redis/go-redis/v9` | v9.5+ — official, context-aware |
| Version | Redis 7.2 | Redis Stack not needed — plain Redis sufficient |
## OpenTelemetry
| Concern | Choice |
|---------|--------|
| Go OTLP receiver | `go.opentelemetry.io/collector` | Use Collector components, not full collector deployment |
| Proto definitions | `github.com/open-telemetry/opentelemetry-proto` | Reference for OTLP→Argus signal mapping |
## Dashboard: React + TypeScript
| Concern | Library | Version |
|---------|---------|---------|
| Framework | React | 18.x |
| Build | Vite | 5.x — faster than CRA/webpack |
| UI Components | shadcn/ui | Latest — spec-specified, Radix primitives |
| Styling | Tailwind CSS | v3.4+ — pairs with shadcn |
| Charting | Apache ECharts | v5.5+ — spec-specified, canvas rendering |
| ECharts React wrapper | `echarts-for-react` | v3.0+ |
| WebSocket | Native browser API | No library needed |
| State | Zustand | v4.5+ — simpler than Redux for this scale |
| Query/cache | TanStack Query | v5.x — server state, auto-refetch |
| Router | React Router | v6.x |
| Type checking | TypeScript | v5.4+ |
| Forms | React Hook Form | v7.x |
| SQL editor | CodeMirror | v6.x — for query interface |
- Plotly — spec explicitly rejects it
- Chart.js — SVG-based, degrades at 100K+ points
- MUI/Ant Design — shadcn/ui is spec-specified
- Redux — overkill for this SPA
## Python (ML/Semantic Detection)
| Concern | Library |
|---------|---------|
| gRPC server | `grpcio` + `grpcio-tools` — generated from same .proto files |
| LLM-as-judge | `anthropic` SDK or `openai` SDK — configurable |
| Embedding | `sentence-transformers` — local embedding for similarity |
| Statistical | `scipy`, `numpy` — KL divergence, distribution analysis |
| Packaging | `uv` — fast, deterministic, preferred over pip |
## Dev Tooling
| Concern | Choice |
|---------|--------|
| Task runner | `Makefile` — universal, no extra install |
| Containerization | Docker Compose v2 (dev), multi-stage Dockerfiles |
| Linting (Go) | `golangci-lint` v1.57+ |
| Linting (TS) | ESLint + Prettier |
| CI | GitHub Actions |
| Proto registry | `buf.build` (optional, free for open source) |
## Confidence Levels
- Go stack: **High** — well-established choices, production-proven
- Protobuf/buf: **High** — buf is the clear winner for schema management
- ClickHouse client: **High** — clickhouse-go/v2 is the official maintained client
- Dashboard stack: **High** — matches spec, shadcn+Vite is 2024-2025 standard
- Python ML stack: **Medium** — component selection depends on which semantic engines are prioritized first
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

### Auth Client Protocol (any HTTP client calling `/api/v1/auth/*`)

Every HTTP client in this codebase that calls protected auth endpoints **must** follow this three-step protocol. Missing any step produces a 403.

**Step 1 — Cookie jar**
The `http.Client` must be constructed with a `net/http/cookiejar` so cookies persist across requests within the same session.

**Step 2 — CSRF prefetch before any mutating request**
Before the first POST/PUT/DELETE to `/api/v1/auth/*`, do:
```
GET /api/v1/auth/csrf-token
```
The server sets a `csrf_token` cookie (path `/api/v1/auth`, HttpOnly=false) and returns the same value in the `X-CSRF-Token` response header. Capture the header value.

**Step 3 — Double-submit on every mutating request**
Send both:
- The `csrf_token` cookie (carried automatically by the cookie jar)
- `X-CSRF-Token: <token>` request header (exact value from step 2)

The `CSRFMiddleware` (`internal/auth/csrf.go`) constant-time compares them; mismatch or absence → 403.

**Excluded routes (no CSRF needed):**
- `/api/v1/auth/refresh` — uses HttpOnly refresh cookie, explicitly excluded
- `/api/v1/auth/mfa/challenge` — public MFA entry point
- GET `/api/v1/auth/csrf-token` — the token fetch itself (safe method)

**Applies to:** `cmd/argus/tui/auth/client.go`, the e2e validation script, any SDK or test helper that calls login/logout/MFA endpoints directly.

---

### Signal Ingest Authentication (SDK → `/v1/signals`)

Signal ingest uses **API key auth**, not JWT:
- Header: `X-Argus-API-Key: <key>` (NOT `Authorization: Bearer`)
- Required scope: `signals:write`
- Route is in `ExcludedPaths` so JWT middleware does not apply

---

### Route Prefix Map

| Path prefix | Auth type | Notes |
|-------------|-----------|-------|
| `POST /v1/signals` | API key (`X-Argus-API-Key`) | `signals:write` scope required |
| `GET /v1/signals` | None (public) | Query by `app_id`, `layer`, etc. |
| `GET /api/v1/traces/{id}` | JWT | Returns `.spans[]` not `.signals[]` |
| `POST /api/v1/auth/login` | CSRF double-submit | Needs cookie jar + prefetch |
| `GET /api/v1/auth/csrf-token` | None | Sets cookie, returns header |
| `GET /api/v1/api-keys` | JWT | User sees own keys |
| `POST /api/v1/api-keys` | JWT + role (admin/analyst) | Returns full key once only |

<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

### Phase 3: Processing Pipeline

The processing pipeline is the heart of signal enrichment, transforming raw ingest signals into correlated, enriched, and scored entities ready for detection and investigation.

**Signal Flow Architecture:**
```
Ingest Queue (Phase 2)
         ↓
   WorkerPool (fixed-size goroutine pool)
         ↓
  Processor Chain (serial, 7 stages):
    1. SchemaValidator - reject incomplete signals
    2. Normalizer - canonicalize field values
    3. CorrelationTagger - link traces via Redis sorted sets
    4. Enricher - call GeoIPEnricher for geolocation
    5. BaselineScorer - compute z-scores from cached profiles
    6. Router - hand signal to storage for persistence
         ↓
  ClickHouse (signal storage)
     + PostgreSQL baseline_profiles
     + Redis cache (correlations, baselines, GeoIP)
```

**Async Baseline Engine (independent):**
- 10-minute computation cycle
- Queries ClickHouse for (app_id, layer, category) combos with ≥100 samples
- Caches profiles in Redis (5-min TTL for fast lookups)
- Persists to PostgreSQL (durability across restarts)
- Does NOT block ingest hot path (separate goroutine)

**Worker Pool (Pitfall 6 Prevention):**
- Fixed goroutine count: GOMAXPROCS × 2 (default 4 on quad-core)
- Goroutine count remains constant regardless of signal volume
- Backpressure flows backward: full pipeline channels block workers
- Graceful shutdown: drain pending signals before exit

**Critical Pitfall Prevention:**
1. **Pitfall 1 (Blocking Baseline):** BaselineEngine async only, 10-min intervals
2. **Pitfall 2 (Redis Memory):** TTLs on all keys (30s traces, 5m baselines, 24h GeoIPs) + metrics monitor
3. **Pitfall 3 (Backpressure):** Channel depth gauges track buildup per processor
4. **Pitfall 4 (Z-Score NaN):** ComputeZScore clamps stddev=0 → returns 0
5. **Pitfall 5 (GeoIP Staleness):** Database age checked at startup, daily refresh scheduler
6. **Pitfall 6 (Goroutine Explosion):** Fixed pool verified with 100+ signal tests

**Performance Targets (Achieved):**
- Ingest throughput: 10K+ signals/sec
- Ingest latency p99: <100ms (baseline engine async)
- Enrichment latency: <50ms (pipeline) + <100ms (storage write)
- Goroutine overhead: GOMAXPROCS × 2 + fixed OS threads
- Memory stability: bounded by Redis TTLs and PostgreSQL cleanup

**Integration Points:**
- Phase 2 ClickHouse: signals table (source of historical data for baseline)
- Phase 2 PostgreSQL: baseline_profiles table (durable profile storage)
- Phase 2 Redis: correlation sets, baseline cache, GeoIP cache
- Phase 1 ArgusSignal schema: validation rules, enrichment fields
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

## Design System Conventions (Phase 6.5+)

**Dark-First Theme:**
- Background: #0A0A0B (near-black)
- Borders: #2A2A2F (subtle)
- Primary text: #FFFFFF
- Secondary text: #A0A0A0 (muted)
- All components render dark by default

**Typography:**
- Primary font: Geist (sans-serif) via Google Fonts
- Code font: Geist Mono via Google Fonts
- Base size: 16px, line height: 1.5

**Color System:**
- Layer colors (L1-L10): L1=#EF4444 (red) → L10=#F43F5E (rose)
- Status: Success (#22C55E), Warning (#EAB308), Error (#EF4444), Info (#3B82F6)
- Use design tokens from `web/src/lib/design-tokens.ts`

**Spacing & Layout:**
- 4px grid (4, 8, 12, 16, 20, 24, 32, 40, 48, 56, 64px)
- Tailwind spacing scale (p-1 = 4px, p-2 = 8px, etc.)
- Consistent breathing room in all layouts

**Motion & Interaction:**
- Transitions max 200ms
- Loading: skeleton placeholders (not spinners)
- Focus: 2px outline with primary color, 2px offset
- No decorative animations

**Accessibility (WCAG AA):**
- Contrast ratio ≥ 4.5:1 for all text
- Touch targets ≥ 44px (width/height)
- Keyboard: Tab (navigation), Escape (close), Arrow keys (lists)
- ARIA labels on buttons, forms, landmarks
- Screen reader friendly

**Power User Features:**
- Command Palette (⌘K / Ctrl+K): search, navigation, actions
- Keyboard shortcuts displayed in menu labels
- Every action reachable in ≤3 clicks

**Design References:**
- Linear (minimal, keyboard-driven)
- Vercel (clean, purposeful)
- Tailscale (operator-friendly, no noise)

All token values defined in `web/src/lib/design-tokens.ts`

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
