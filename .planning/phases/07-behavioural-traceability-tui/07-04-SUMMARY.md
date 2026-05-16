---
phase: 07-behavioural-traceability-tui
plan: 04
subsystem: api
tags: [go, chi, jwt, rbac, clickhouse, postgresql, behaviour, trace, baseline, drift]

requires:
  - phase: 07-02
    provides: RunReconstructor, TimelineBuilder, RunGraph, SessionTimeline types
  - phase: 07-03
    provides: SessionProfileStore, SessionBaselineEngine, ComputeSessionDrift

provides:
  - "GET /api/v1/traces/{traceID}/graph — causal span tree as RunGraph JSON"
  - "GET /api/v1/sessions/{sessionID}/timeline — session-scoped signal timeline"
  - "GET /api/v1/conversations/{conversationID}/behaviour — timeline + drift_score"
  - "GET /api/v1/alerts/{alertID}/chain — alert causal chain via RunGraph"
  - "GET /api/v1/traces/recent — RecentRunSummary list for TUI run list"
  - "internal/api/behaviour/ package: BehaviourHandler with RegisterRoutes"
  - "SessionBaselineEngine started alongside API server"

affects: [07-05, 07-06, tui-behaviour, dashboard-integration]

tech-stack:
  added: []
  patterns:
    - "behaviour package pattern: handler struct with injected driver.Conn + pgxpool.Pool + domain objects"
    - "RegisterRoutes delegation: handler owns route registration, caller wraps with auth middleware"
    - "Dual-store drift: app_id from ClickHouse lookup -> SessionProfileStore.Get -> ComputeSessionDrift"
    - "Alert chain resolution: PostgreSQL signal_ids -> ClickHouse trace_id -> RunReconstructor.BuildRun"
    - "Global JWT middleware + per-group RequireRole for role-specific endpoint gating"

key-files:
  created:
    - internal/api/behaviour/handler.go
    - internal/api/behaviour/handler_test.go
    - internal/api/behaviour/recent.go
  modified:
    - cmd/argus/api.go

key-decisions:
  - "Used ch.Conn() accessor to get driver.Conn from *storage.ClickHouse rather than passing the struct"
  - "RegisterRoutes uses r.Group + RequireRole since JWT is already applied globally via AuthMiddleware"
  - "drift_score is null (not 0) when no session baseline profile exists — communicates 'not computable' vs 'no drift'"
  - "recent.go uses Nullable(Float32) scan into *float32 to handle rows with no deviation data"
  - "ServeAlertChain defers ancestor-only filtering per RESEARCH.md partial-success definition"

patterns-established:
  - "Behaviour endpoint pattern: URLParam -> domain call -> ErrXxxNotFound -> 404, else 500 or 200+JSON"

requirements-completed: [REQ-P7-06, REQ-P7-07, REQ-P7-08, REQ-P7-09]

duration: 4min
completed: 2026-05-16
---

# Phase 07 Plan 04: Behavioural Traceability HTTP Endpoints Summary

**Five JWT-gated endpoints exposing Phase 7 trace/session/behaviour/alert data via internal/api/behaviour/ package registered in cmd/argus/api.go under RequireRole("analyst","admin")**

## Performance

- **Duration:** 4 min
- **Started:** 2026-05-16T14:48:35Z
- **Completed:** 2026-05-16T14:52:19Z
- **Tasks:** 3 (Tasks 1+2 shipped together; Task 3 API wiring)
- **Files modified:** 4

## Accomplishments

- Created `internal/api/behaviour/` package with `BehaviourHandler` struct owning 5 `http.HandlerFunc` methods
- `ServeConversationBehaviour` chains ClickHouse app_id lookup -> SessionProfileStore.Get -> ComputeSessionDrift producing drift_score
- `ServeAlertChain` resolves PostgreSQL signal_ids -> ClickHouse trace_id -> RunReconstructor.BuildRun
- `ServeRecentRuns` aggregates signals by trace_id (groupUniqArray, min/max timestamp, peak deviation) powering the TUI run list
- `SessionBaselineEngine` started async alongside the API server in `cmd/argus/api.go`
- All 5 endpoints gated: global JWT via existing `AuthMiddleware` + per-group `RequireRole("analyst","admin")`
- `go build ./...` and `go test ./internal/api/behaviour/...` both exit 0

## Task Commits

1. **Tasks 1+2: BehaviourHandler + recent.go** - `cec2ff2` (feat)
2. **Task 3: Wire into api.go with auth** - `382e098` (feat)

## Files Created/Modified

- `internal/api/behaviour/handler.go` - BehaviourHandler struct, NewBehaviourHandler, RegisterRoutes, 4 core handlers, writeJSON/writeErr helpers
- `internal/api/behaviour/handler_test.go` - compile-time smoke test asserting all 5 HandlerFunc signatures
- `internal/api/behaviour/recent.go` - RecentRunSummary type, recentRunsSQL constant, ServeRecentRuns handler
- `cmd/argus/api.go` - added imports for behaviour/trace/baseline; instantiate reconstructor + timelineBuilder + sessionStore + behaviourHandler; start SessionBaselineEngine; register r.Group with RequireRole

## Decisions Made

- Used `ch.Conn()` to extract `driver.Conn` from `*storage.ClickHouse` — this is the accessor the storage package exposes
- JWT is already applied globally via `auth.AuthMiddleware` in api.go; behaviour routes just add `RequireRole("analyst","admin")` via `r.Group`
- `drift_score` JSON field is `null` (not absent, not zero) when no baseline profile exists — encodes "not computable" semantics
- `recent.go` scans `max(enrich_baseline_deviation)` into `*float32` to handle NULL rows without panicking
- Alert chain returns full RunGraph without ancestor-only filtering — per RESEARCH.md this is acceptable for first pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created recent.go before handler.go could compile**
- **Found during:** Task 1 (BehaviourHandler compilation)
- **Issue:** `RegisterRoutes` in handler.go references `h.ServeRecentRuns` which lives in recent.go; without that file the package would not compile even for the RED test step
- **Fix:** Created recent.go (Task 2 content) alongside handler.go before running the first build
- **Files modified:** internal/api/behaviour/recent.go
- **Verification:** `go build ./internal/api/behaviour/...` exits 0; test passes
- **Committed in:** cec2ff2 (combined Task 1+2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 3 - blocking compilation dependency)
**Impact on plan:** No scope change. Tasks 1 and 2 were committed together since they form an indivisible compilation unit.

## Issues Encountered

None beyond the blocking compilation dependency handled above.

## Known Stubs

None — all handlers call real domain functions (RunReconstructor, TimelineBuilder, SessionProfileStore, ComputeSessionDrift). drift_score being null for new deployments (no baseline yet computed) is expected behaviour, not a stub.

## Next Phase Readiness

- Phase 07-05 (TUI behaviour screens) can now call all 5 endpoints
- Phase 07-06 (integration tests) has real endpoints to test against
- SessionBaselineEngine is running — drift_score will become non-null once 10+ minutes of session data accumulates

---
*Phase: 07-behavioural-traceability-tui*
*Completed: 2026-05-16*
