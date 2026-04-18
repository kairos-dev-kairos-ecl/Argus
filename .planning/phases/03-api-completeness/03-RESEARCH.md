# Phase 3: API Completeness - Research

**Researched:** 2026-04-17
**Domain:** Go HTTP API, PostgreSQL CRUD, WebSocket streaming, JWT auth, ClickHouse queries
**Confidence:** HIGH — all findings derived directly from codebase inspection and existing code patterns

---

## Summary

The codebase is significantly more advanced than the AUDIT.md and ROADMAP.md imply. Most missing endpoints exist in some form — they are split between fully-implemented handlers (auth, layer status, trace, query, alerts, users) and stub-returning-501 handlers (rules, apps). The real work in Phase 3 is not building endpoints from scratch but: (1) replacing the 8 stubs in `handler_stubs.go` with real implementations backed by the PostgreSQL `detection_rules` and `apps` tables, (2) fixing a critical mismatch between `handler_rules.go` (which has `ServeListRules`/`ServeCreateRule` with correct logic) and `handler_stubs.go` (which has `handleListRules`/`handleCreateRule` returning 501), (3) adding missing incident CRUD endpoints for `handler_incidents.go`, and (4) ensuring the `ClickHouse` column layout in `SignalsInsertColumns` matches the expanded `schema.go` DDL (a critical drift bug).

**Primary recommendation:** The scaffolding already exists. Phase 3's job is to wire the stub methods to real storage, fix the handler naming conflict between `handler_rules.go` and `handler_stubs.go`, and reconcile the ClickHouse insert column list with the full schema DDL before any new code is written.

---

## Critical Pre-Work Findings (Must Fix Before Any New Endpoint Work)

### Finding 1: ClickHouse Column Drift — BLOCKING

`internal/storage/clickhouse.go` `SignalsInsertColumns` lists ~57 columns.
`internal/storage/schema.go` `SignalsTableDDL` defines ~200+ columns (the full Phase 1 schema rewrite columns).

The `signalToClickHouseRow` function only populates ~14 layer context fields (L1 3 fields, L2 3, L3 3, L4 2, L5 3, L6 3, L7 4, L8 3, L9 4, L10 2) from the old abbreviated schema. The DDL now has 24 L1 columns, 18 L2 columns, 22 L3 columns, 24 L4 columns, 24 L5 columns, 22 L6 columns, 25 L7 columns, 25 L8 columns, 25 L9 columns, 24 L10 columns.

**Effect:** Every signal INSERT will fail with a column count mismatch. This must be reconciled before Phase 3 endpoint work is tested end-to-end.

**Action:** `SignalsInsertColumns` and `signalToClickHouseRow` must be updated to cover all DDL columns OR the DDL must be rolled back to match the insert list. Given Phase 1 is "planned" but the DDL has already been updated, the correct fix is to update the insert code to cover all columns (passing zero values for unimplemented proto fields).

### Finding 2: Duplicate Rule Handlers — CONFUSING

`handler_rules.go` defines `ServeListRules`, `ServeCreateRule`, `ServeDeleteRule` (real logic, uses `h.store`).
`handler_stubs.go` defines `handleListRules`, `handleCreateRule`, `handleDeleteRule`, etc. (all return 501).
`receiver_query.go` routes to `handleListRules`, `handleCreateRule`, etc. (the 501 stubs).

The real handlers in `handler_rules.go` are never called — they use a different naming convention (`ServeXxx` vs `handleXxx`) and are not registered in `RegisterRoutes`.

**Action:** Wire `handleListRules` → `h.ServeListRules`, `handleCreateRule` → `h.ServeCreateRule`, etc. OR move the logic inline. Do not duplicate.

### Finding 3: `handler_incidents.go` Status Unknown

`receiver_query.go` registers `handleGetIncident`, `handleAcknowledgeIncident`, `handleResolveIncident` but these appear in `handler_incidents.go` (file exists). Need to verify contents before planning incident work.

---

## Current Handler State Inventory

| Endpoint | File | Status |
|----------|------|--------|
| `GET /v1/signals` | receiver_query.go | DONE — cursor pagination, all filters |
| `GET /v1/schema/signals` | receiver_query.go | DONE |
| `GET /api/v1/layers/status` | receiver_query.go | DONE — queries ClickHouse 5-min window |
| `GET /v1/signals/stream` | receiver_ws.go | DONE — gorilla/websocket, ping/pong, broadcaster |
| `GET /api/v1/traces/{traceId}` | receiver_query.go | DONE — returns all signals for trace |
| `POST /api/v1/query` | receiver_query.go | DONE — DDL blocked, 30s timeout |
| `GET /api/v1/rules` | handler_stubs.go | STUB (501) — real logic exists in handler_rules.go, not wired |
| `POST /api/v1/rules` | handler_stubs.go | STUB (501) — real logic exists in handler_rules.go, not wired |
| `GET /api/v1/rules/{id}` | handler_stubs.go | STUB (501) — no real implementation found |
| `PUT /api/v1/rules/{id}` | handler_stubs.go | STUB (501) — no real implementation found |
| `DELETE /api/v1/rules/{id}` | handler_stubs.go | STUB (501) — real logic exists in handler_rules.go, not wired |
| `POST /api/v1/rules/validate` | handler_stubs.go | STUB (501) |
| `POST /api/v1/rules/test` | handler_stubs.go | STUB (501) |
| `GET /api/v1/alerts` | handler_alerts.go | DONE — queries PostgreSQL `alerts` table |
| `GET /api/v1/alerts/{id}` | handler_alerts.go | DONE |
| `POST /api/v1/alerts/{id}/acknowledge` | handler_alerts.go | DONE |
| `GET /api/v1/incidents` | handler_incidents.go | UNKNOWN — file exists, contents not verified |
| `GET /api/v1/incidents/{id}` | handler_incidents.go | UNKNOWN |
| `POST /api/v1/incidents/{id}/acknowledge` | handler_incidents.go | UNKNOWN |
| `POST /api/v1/incidents/{id}/resolve` | handler_incidents.go | UNKNOWN |
| `POST /api/v1/auth/login` | handler_auth.go | DONE — RS256 JWT, refresh cookie |
| `POST /api/v1/auth/refresh` | handler_auth.go | DONE — token rotation |
| `POST /api/v1/auth/logout` | handler_auth.go | DONE — session + token revocation |
| `POST /api/v1/auth/setup` | handler_auth.go | DONE — first-user setup |
| `GET /api/v1/users` | handler_users.go | DONE — sanitized list (no password hashes) |
| `POST /api/v1/users` | handler_users.go | DONE (partial — needs verification) |
| `GET /api/v1/apps` | handler_stubs.go | STUB (501) |
| `POST /api/v1/apps` | handler_stubs.go | STUB (501) |
| `GET /api/v1/apps/{id}/key` | handler_stubs.go | STUB (501) |
| `POST /api/v1/apps/{id}/key/rotate` | handler_stubs.go | STUB (501) |
| `GET /api/v1/audit` | handler_audit.go | UNKNOWN — file exists |

---

## PostgreSQL Schema State

### Migrations Already Applied

| File | Tables Created |
|------|---------------|
| `007_auth.up.sql` | `users` (extended), `sessions`, `audit_log`, `token_revocations` |
| `008_core_tables.up.sql` | `apps`, `detection_rules`, `alerts`, `incidents`, `notification_channels`, `routing_rules`, `suppression_rules` |

**All required PostgreSQL tables exist.** The migration gap listed in AUDIT.md is closed by `008_core_tables.up.sql`.

**Key schema detail for apps handler:** The `apps` table stores API keys in plain `api_key TEXT` with a `api_key_prefix TEXT`. The existing `AuthValidator.validateAPIKey()` (in `internal/ingest/auth.go`) presumably queries this table. The handler_stubs.go apps handlers need to INSERT into this table and generate the API key, hash it, and return the plaintext key ONCE (only at creation time).

---

## Architecture Patterns

### Pattern 1: Handler Registration (Established)

All handlers are methods on `*QueryHandler`. Route registration happens in `RegisterRoutes(mux *chi.Mux)` in `receiver_query.go`. New handlers follow this exact pattern — no separate router files.

```go
// In receiver_query.go RegisterRoutes():
mux.Get("/api/v1/apps", h.handleListApps)
mux.Post("/api/v1/apps", h.handleCreateApp)
```

### Pattern 2: Graceful Degradation Pattern (Established)

All handlers check for required service availability before processing:

```go
func (h *QueryHandler) handleListApps(w http.ResponseWriter, r *http.Request) {
    if h.pg == nil {  // or h.alertAvailable(), h.authAvailable()
        jsonError(w, "storage unavailable", http.StatusServiceUnavailable)
        return
    }
    // ...
}
```

The `QueryHandler` currently has no direct `pg *pgxpool.Pool` field — it gets PostgreSQL access through `h.alertRouter.pool`. New CRUD handlers for apps/rules should get pool access the same way, OR a `pool` field should be added directly to `QueryHandler`.

**Recommendation:** Add `pool *pgxpool.Pool` directly to `QueryHandler` and wire it in `api.go`. The `alertRouter.pool` is an indirect access pattern that is fragile.

### Pattern 3: JSON Error Helper (Established)

```go
func jsonError(w http.ResponseWriter, msg string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
```

Use this everywhere. Never call `http.Error()` (returns plain text, not JSON).

### Pattern 4: WebSocket — Already Implemented

The WebSocket implementation in `receiver_ws.go` is complete and production-quality:
- gorilla/websocket upgrader (CheckOrigin: allow all in dev)
- 30s ping / 60s read deadline (pong resets read deadline)
- SignalBroadcaster fan-out (subscribe/unsubscribe per connection)
- `r.Context().Done()` for clean shutdown

The frontend (`useSignalStream.ts`) connects to `ws://host/v1/signals/stream` which maps to `handleStream`. This is already wired in both `api.go` and `ingest.go`. **WebSocket is done — no work needed.**

### Pattern 5: Auth Middleware Chain (Established)

```go
// Global middleware in api.go:
r.Use(func(next http.Handler) http.Handler {
    return auth.AuthMiddleware(auth.MiddlewareConfig{
        ExcludedPaths: map[string]bool{
            "/health": true,
            "/api/v1/auth/login": true,
            "/api/v1/auth/refresh": true,
            // ...
        },
    })(next)
})

// Per-route RBAC (for admin-only endpoints):
mux.With(auth.RequireRole("admin")).Delete("/api/v1/users/{id}", h.handleDeleteUser)
```

`auth.RequireRole()` and `auth.RequirePermission()` middleware exist and are production-ready.

### Pattern 6: ClickHouse Named Parameters (Established)

```go
// Safe pattern — use named params for user-supplied strings:
rows, err := h.ch.Conn().Query(ctx, `SELECT ... WHERE app_id = {app_id:String}`,
    clickhouse.Named("app_id", appID))

// Safe for integers — interpolate directly from parsed int64:
query += ` AND layer = ` + strconv.FormatInt(int64(layer), 10)
```

Never interpolate user-supplied strings directly into ClickHouse queries.

---

## Frontend Contract (What API Responses Must Look Like)

### `GET /api/v1/layers/status` Response

Frontend `LayerStatus` interface:
```typescript
interface LayerStatus {
  layer: Layer          // 'L1_HARDWARE' | ... | 'L10_APPLICATION'
  status: 'green' | 'yellow' | 'gray' | 'red'
  last_signal_time: string | null   // ISO8601 or null
  signal_count_5min: number
  error_message?: string
}
```

**Current backend response does NOT match.** The backend returns `{ layer: 1, name: "Hardware", signal_count: 42, last_seen_at: "...", status: "active" }`. The frontend expects `{ layer: "L1_HARDWARE", status: "green", last_signal_time: "...", signal_count_5min: 42 }`.

**Action required:** Update `handleGetLayerStatus` to:
1. Return `layer` as string enum (`"L1_HARDWARE"` etc.), not integer
2. Map `status: "active"` → `"green"`, `"idle"` (last seen < 5min) → `"yellow"`, `"unknown"` → `"gray"`
3. Return field named `signal_count_5min` not `signal_count`
4. Return field named `last_signal_time` not `last_seen_at`

### `GET /api/v1/traces/{traceId}` Response

Frontend `Trace` interface:
```typescript
interface Trace {
  trace_id: string
  spans: Span[]          // NOT signals — the frontend expects Span[] not ArgusSignal[]
  detections: Detection[]
  duration_ms: number
}
interface Span {
  signal_id: string
  layer: Layer           // string enum
  start_time: string     // ISO8601
  duration_ms: number
  parent_signal_id?: string
  status: 'ok' | 'error'
  message: string
}
```

**Current backend returns** `{ trace_id: "...", signals: [ArgusSignal...] }`. Frontend expects `{ trace_id: "...", spans: [Span...], detections: [], duration_ms: N }`.

**Action required:** Map `ArgusSignal[]` → `Span[]` in the response:
- `span.signal_id` = `signal.signal_id`
- `span.layer` = layer enum string (e.g., `"L5_OUTPUT_DECODING"`)
- `span.start_time` = `signal.timestamp` as ISO8601
- `span.duration_ms` = `signal.duration_ms` or 0
- `span.parent_signal_id` = `signal.parent_span_id`
- `span.status` = `"error"` if severity >= 4, else `"ok"`
- `span.message` = `signal.category`
- `trace.duration_ms` = max(timestamp) - min(timestamp) across all spans
- `trace.detections` = `[]` (Phase 4 will populate from detection engine)

### `POST /api/v1/query` Request/Response

Frontend sends: `{ sql: "SELECT ...", limit: 10000 }`
Frontend expects: `QueryResult { rows: Record<string, unknown>[], cursor?: string, total: number, execution_time_ms: number }`

**Current backend returns:** `{ columns: [], rows: [][], row_count: N }`

**Mismatch:** Frontend expects `rows` as `Record<string, unknown>[]` (objects keyed by column name), backend returns `rows` as `[][]` (arrays). Also missing `total` and `execution_time_ms` fields.

**Action required:** Update `handlePostQuery` to return row objects (zip columns + values into maps) and add `total` and `execution_time_ms` fields.

---

## Apps Handler Implementation Guide

The `apps` CRUD handlers are the most significant stubs. Implementation pattern:

```go
// POST /api/v1/apps — create app + generate API key
func (h *QueryHandler) handleCreateApp(w http.ResponseWriter, r *http.Request) {
    // 1. Parse { name, description }
    // 2. Generate API key: "arg_" + 32 random bytes hex
    // 3. Compute SHA256 hash of full key
    // 4. Store hash in DB (never plaintext)
    // 5. Return plaintext key ONCE in response
    // INSERT INTO apps (name, description, api_key, api_key_prefix, created_by)
    //   VALUES ($1, $2, $3, $4, $5)
    // Note: api_key column stores the HASH, api_key_prefix stores first 8 chars for display
}

// GET /api/v1/apps — list apps (never return api_key hash)
// GET /api/v1/apps/{id}/key — show api_key_prefix only, never full key
// POST /api/v1/apps/{id}/key/rotate — generate new key, update hash in DB, return new plaintext once
```

**Schema note:** `008_core_tables.up.sql` has `api_key TEXT NOT NULL UNIQUE` — this should store the SHA256 hash. The column name is misleading; it stores the hash, not the raw key.

---

## Rules Handler Implementation Guide

The `detection_rules` table stores rule config as JSONB. The existing `engine.Rule` struct is the Go type. Implementation:

```go
// GET /api/v1/rules — query detection_rules table, unmarshal config JSONB to engine.Rule
// POST /api/v1/rules — validate rule, INSERT into detection_rules, add to in-memory h.store
// GET /api/v1/rules/{id} — SELECT by id
// PUT /api/v1/rules/{id} — UPDATE config, update h.store
// DELETE /api/v1/rules/{id} — DELETE, remove from h.store

// The existing handler_rules.go has ServeListRules (in-memory only, no DB persistence)
// The new implementation should do BOTH: persist to DB AND update h.store
```

**Critical:** `handleListRules` in `handler_stubs.go` is what gets called (returns 501). `ServeListRules` in `handler_rules.go` has the in-memory logic but is never called. The stub must be replaced with logic that queries the DB.

---

## Layer Status Query — Performance

Current implementation queries ClickHouse live on every request:
```sql
SELECT layer, count() AS cnt, max(timestamp) AS last_seen
FROM signals
WHERE timestamp >= now() - INTERVAL 5 MINUTE
GROUP BY layer ORDER BY layer
```

This is a 5-minute rolling window aggregation with GROUP BY — ClickHouse handles this efficiently with partition pruning on `toYYYYMM(timestamp)`. For the 30-second frontend poll interval with typical signal volumes (10K/sec), this query will hit the index. **No caching needed for Phase 3** — add Redis caching in Phase 5 if performance becomes an issue.

---

## Health Endpoint Gap

Current `/health` only checks ClickHouse. The frontend has no dependency on health endpoint content, but the docker-compose healthcheck calls it. Production-grade requirement: check all components.

```go
// Full component health check:
// - ClickHouse: conn.Ping() with 3s timeout
// - PostgreSQL: pool.Ping() with 3s timeout
// - Redis: client.Ping() with 3s timeout
// Overall: "healthy" if all pass, "degraded" if any fail, components map shows individual status
```

The `makeHealthHandler` function currently only takes `*storage.ClickHouse`. It needs to also accept `*pgxpool.Pool` and `*redis.Client`. The function signature in `api.go` must be updated.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| API key generation | Custom entropy | `crypto/rand` + hex encoding | Cryptographically secure |
| Password hashing | Custom hash | `auth.UserService.CreateUser()` (uses bcrypt internally) | Already implemented in auth package |
| JWT issuance | Custom JWT | `auth.TokenManager.IssueAccessToken()` | Already exists, RS256 |
| Session management | Custom session table | `auth.SessionManager.CreateSession()` | Already exists |
| WebSocket | Custom upgrade | gorilla/websocket (already imported in receiver_ws.go) | Already wired |
| SQL injection prevention | Manual escaping | ClickHouse named params, PostgreSQL `$N` params | Already established pattern |
| Rule validation | Custom YAML parser | `engine.Rule.Validate()` | Already exists |
| RBAC | Custom middleware | `auth.RequireRole()`, `auth.RequirePermission()` | Already exists |

---

## Common Pitfalls

### Pitfall 1: Column Count Mismatch on ClickHouse INSERT

**What goes wrong:** `signalToClickHouseRow` calls `batch.Append(...)` with 57 values but the DDL now has 200+ columns. Every INSERT will fail.
**Why it happens:** Phase 1 expanded the DDL schema but didn't update the insert code.
**How to avoid:** Before writing any new handler, update `SignalsInsertColumns` and `signalToClickHouseRow` to match the DDL exactly. Use a column count assertion test.
**Warning signs:** `batch.Append()` returns error mentioning column count mismatch.

### Pitfall 2: Frontend Field Name Mismatch

**What goes wrong:** Endpoint returns `{ signal_count: 42 }` but frontend reads `signal_count_5min` → shows 0.
**Why it happens:** Frontend TypeScript types were defined independently from backend response structs.
**How to avoid:** For every endpoint, verify the exact JSON field names against `web/src/types/index.ts` before marking done.
**Warning signs:** Frontend shows loading spinner indefinitely or empty data despite 200 responses.

### Pitfall 3: Stub vs. Real Handler Routing Confusion

**What goes wrong:** Fix is applied to `handler_rules.go:ServeListRules` but `receiver_query.go` routes to `handleListRules` (the stub). Appears fixed but still returns 501.
**How to avoid:** `handler_stubs.go` stubs must be fully deleted or replaced in-place. The canonical route targets are in `RegisterRoutes` — always confirm the method name matches.

### Pitfall 4: PostgreSQL Pool Access

**What goes wrong:** Apps/rules handlers need `*pgxpool.Pool` but `QueryHandler` only exposes it indirectly via `h.alertRouter.pool`. If `alertRouter` is nil (no PostgreSQL), the handler panics.
**How to avoid:** Add `pool *pgxpool.Pool` directly to `QueryHandler`. Wire it in `api.go` when PostgreSQL is available. All handlers use `h.pool` directly with nil check.

### Pitfall 5: API Key Return Policy

**What goes wrong:** API key returned on GET /api/v1/apps after creation (security leak).
**Why it happens:** Easy to forget the "return key once on creation" requirement.
**How to avoid:** `GET /api/v1/apps` and `GET /api/v1/apps/{id}` NEVER return the key or hash. Only `POST /api/v1/apps` (create) and `POST /api/v1/apps/{id}/key/rotate` return the plaintext key — and only once.

### Pitfall 6: ClickHouse `FINAL` on Layer Status Query

**What goes wrong:** Forgetting `FINAL` on ReplacingMergeTree queries returns duplicate rows from pending merges.
**Note:** The layer status query does `FROM signals WHERE timestamp >= now() - INTERVAL 5 MINUTE` without FINAL. For COUNT purposes this is acceptable (minor overcounting). The trace query correctly uses `FINAL`. Leave layer status without FINAL — the overhead of FINAL on the entire table is not worth it for aggregate counts.

### Pitfall 7: ClickHouse `timestamp` Column Type

The `timestamp` column is `DateTime64(9)` (nanoseconds). When filtering with `parseDatetime64BestEffort()`, the function is correct. When using `now() - INTERVAL 5 MINUTE`, ClickHouse resolves this correctly against DateTime64. No action needed.

---

## Routing Architecture Decision

**Decision: Keep all handlers on `QueryHandler`, all routes in `RegisterRoutes`.** Do not create new handler structs or route files.

Rationale: The existing pattern (`api.go` creates `QueryHandler`, calls `RegisterRoutes`) is clean and the codebase follows it consistently. Creating a separate `AppsHandler` or splitting routes would require modifying `api.go` and adding new bootstrap code. The `QueryHandler` is already a "platform handler" holding references to all services.

**What changes in `api.go`:** Pass `pgPool` directly to `QueryHandler` via a new `SetPool` setter, parallel to how `SetAuthService`, `SetAlertRouter`, and `SetRuleStore` work.

---

## Migration Strategy

**Migrations 007 and 008 already exist and cover all required tables.** No new migration files needed for Phase 3 unless schema corrections are required.

If the `api_key` column in `apps` needs to store hash (not plaintext), the current DDL is correct but misleadingly named. No schema change needed — the implementation should hash before storing.

**Migration numbering:** Next migration would be `009_xxx.up.sql` if any DDL changes are discovered during implementation.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| ClickHouse | Signal queries, layer status, trace | Per docker-compose | 24-alpine | Graceful 503 (already implemented) |
| PostgreSQL | Auth, alerts, rules, apps, incidents | Per docker-compose | 16-alpine | Graceful 503 (already patterned) |
| Redis | Alert dedup, incident correlation | Per docker-compose | 7.2-alpine | Graceful degradation |
| gorilla/websocket | WebSocket stream | In go.mod (receiver_ws.go imports it) | Unknown | None needed — already working |

Step 2.6: Environment is Docker Compose — all services assumed available in dev. The application already implements graceful degradation for missing services.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | stdlib `testing` + `testify` |
| Config file | none (go test ./...) |
| Quick run command | `go test ./internal/ingest/... -run TestHandler -v -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase 3 Endpoint Test Map

| Endpoint | Test Type | Automated Command |
|----------|-----------|-------------------|
| `GET /api/v1/layers/status` | smoke (httptest) | `go test ./internal/ingest/... -run TestHandleGetLayerStatus` |
| `GET /v1/signals/stream` (WebSocket) | integration (httptest+ws) | `go test ./internal/ingest/... -run TestWebSocket` (exists in receiver_ws_test.go) |
| `GET /api/v1/traces/{traceId}` | smoke (httptest) | `go test ./internal/ingest/... -run TestHandleGetTrace` |
| `POST /api/v1/query` | unit (httptest) | `go test ./internal/ingest/... -run TestHandlePostQuery` |
| `GET /api/v1/rules` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleListRules` (exists in handler_rules_test.go) |
| `POST /api/v1/rules` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleCreateRule` |
| `GET /api/v1/alerts` | unit (httptest mock DB) | `go test ./internal/ingest/... -run TestHandleListAlerts` |
| `POST /api/v1/alerts/{id}/acknowledge` | unit (httptest mock DB) | `go test ./internal/ingest/... -run TestHandleAcknowledgeAlert` |
| `GET /api/v1/incidents` | unit (httptest mock DB) | `go test ./internal/ingest/... -run TestHandleListIncidents` |
| `POST /api/v1/auth/login` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleLogin` (exists in handler_auth_test.go) |
| `POST /api/v1/auth/refresh` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleRefreshToken` |
| `GET /api/v1/users` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleListUsers` (exists in handler_users_test.go) |
| `GET /api/v1/apps` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleListApps` ❌ Wave 0 |
| `POST /api/v1/apps` | unit (httptest) | `go test ./internal/ingest/... -run TestHandleCreateApp` ❌ Wave 0 |
| Layer status field names | unit | verify JSON field names match TS types |
| Trace response shape | unit | verify `spans[]` not `signals[]` |
| Query response shape | unit | verify `rows` as object array |

### Sampling Rate

- **Per task commit:** `go test ./internal/ingest/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green, all endpoints return non-501

### Wave 0 Gaps (Must Create Before Implementation)

- [ ] `internal/ingest/handler_apps_test.go` — covers apps CRUD (Wave 0: create test file)
- [ ] `internal/ingest/handler_incidents_test.go` — covers incident list/get/ack/resolve (Wave 0)
- [ ] Integration smoke test: `curl http://localhost:8080/api/v1/layers/status` returns correct field names

---

## Implementation Work Breakdown

### Wave 0: Pre-Work (Must Complete First)

1. **Reconcile ClickHouse column drift** — Update `SignalsInsertColumns` and `signalToClickHouseRow` to cover all DDL columns. Until this is done, no signal inserts work.
2. **Add `pool *pgxpool.Pool` to `QueryHandler`** — Add field + `SetPool` method + wire in `api.go`
3. **Verify `handler_incidents.go` contents** — Determine if handlers are real or stubs

### Wave 1: Fix Layer Status + Trace Response Shape

4. **Fix `handleGetLayerStatus`** — Return correct field names and types to match frontend TypeScript
5. **Fix `handleGetTrace`** — Return `{ spans: Span[], detections: [], duration_ms: N }` not `{ signals: ArgusSignal[] }`

### Wave 2: Fix Query Response Shape

6. **Fix `handlePostQuery`** — Return `{ rows: [{},...], total: N, execution_time_ms: N }` with rows as objects

### Wave 3: Replace Rules Stubs

7. **Replace `handleListRules` stub** — Query `detection_rules` table + return from `h.store`
8. **Replace `handleCreateRule` stub** — Validate, INSERT into `detection_rules`, add to `h.store`
9. **Replace `handleGetRule` stub** — SELECT by id
10. **Replace `handleUpdateRule` stub** — UPDATE config, update `h.store`
11. **Replace `handleDeleteRule` stub** — DELETE + remove from `h.store`
12. **Replace `handleValidateRule` stub** — Validate rule structure, return errors (no DB write)
13. **Replace `handleTestRule` stub** — Run rule against last N signals, return match results

### Wave 4: Implement Apps Handlers

14. **Implement `handleListApps`** — Query `apps` table, omit api_key from response
15. **Implement `handleCreateApp`** — Generate key, hash, INSERT, return plaintext key once
16. **Implement `handleGetAppKey`** — Return key prefix only, never full key or hash
17. **Implement `handleRotateAppKey`** — Generate new key, UPDATE hash, return new plaintext once

### Wave 5: Health Endpoint + Prometheus Gaps

18. **Expand health endpoint** — Add PostgreSQL + Redis checks, update `makeHealthHandler` signature
19. **Add missing Prometheus metrics** — WebSocket subscriber gauge, alert generation counter

---

## Sources

### Primary (HIGH confidence — direct codebase inspection)

- `internal/ingest/receiver_query.go` — handler implementations + route registrations
- `internal/ingest/receiver_ws.go` — WebSocket implementation
- `internal/ingest/handler_auth.go` — auth handler implementations
- `internal/ingest/handler_alerts.go` — alert handler implementations
- `internal/ingest/handler_stubs.go` — stub inventory
- `internal/ingest/handler_rules.go` — real rule logic (unwired)
- `internal/storage/schema.go` — full ClickHouse DDL (200+ columns)
- `internal/storage/clickhouse.go` — insert column list (57 columns) — **DRIFT CONFIRMED**
- `internal/auth/middleware.go` — RequireRole, RequirePermission patterns
- `migrations/008_core_tables.up.sql` — all required tables exist
- `web/src/types/index.ts` — exact TypeScript field names
- `web/src/hooks/useSignalStream.ts` — WebSocket URL and message format
- `web/src/hooks/useTrace.ts` — trace API endpoint and expected shape
- `web/src/hooks/useQuery.ts` — query API endpoint and expected response shape
- `cmd/argus/api.go` — service wiring, auth middleware setup

### Secondary (MEDIUM confidence)

- AUDIT.md — point-in-time snapshot (2026-04-12); some items resolved since then

---

## Metadata

**Confidence breakdown:**
- Handler status inventory: HIGH — direct file inspection
- Column drift finding: HIGH — direct comparison of two files in same repo
- Frontend contract: HIGH — TypeScript types and hook implementations read directly
- Migration status: HIGH — migration files read directly
- Pitfalls: HIGH — derived from concrete code evidence

**Research date:** 2026-04-17
**Valid until:** This is an internal codebase research — valid until next code change. Re-read handler files before planning any wave.
