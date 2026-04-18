---
phase: 03-api-completeness
plan: "05"
subsystem: ingest/testing
tags: [tests, response-shape, nil-pool, degradation, regression]
dependency_graph:
  requires: [03-02, 03-03, 03-04]
  provides: [response-shape-tests, apps-degradation-tests]
  affects: [ingest-package-tests]
tech_stack:
  added: []
  patterns: [httptest-unit-tests, nil-pool-degradation-pattern]
key_files:
  created:
    - internal/ingest/handler_response_test.go
    - internal/ingest/handler_apps_test.go
  modified:
    - internal/ingest/receiver_http_test.go
decisions:
  - TestRulesNilPool covers both GET and POST to prove DB-backed wiring on both read and write paths
  - TestCreateAppValidation excluded from scope — requires live PostgreSQL, belongs in integration tests
  - receiver_http_test.go fixed inline (pre-existing bug: missing broadcaster param in NewHTTPReceiver)
metrics:
  duration: "10 minutes"
  completed: "2026-04-18"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 3 Plan 5: API Response Shape and Degradation Tests Summary

**One-liner:** httptest unit tests confirming frontend-compatible JSON field names and DB-backed nil-pool 503 degradation across rules and apps handlers.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Response shape tests for layer status, trace, query, rules | 5922c9e | handler_response_test.go, receiver_http_test.go |
| 2 | Apps handler nil-pool degradation tests | 6b939d8 | handler_apps_test.go |

## What Was Built

### handler_response_test.go (4 tests)

- **TestLayerStatusResponseShape** — Verifies degraded-mode response has `layers[]` with 10 entries, `signal_count_5min` (not `signal_count`), `last_signal_time` (not `last_seen_at`), string layer enum (`L1_HARDWARE`), status `"gray"`, no `"name"` key.
- **TestTraceResponseShape** — Struct serialization test: confirms `"spans"` key (not `"signals"`), `"detections"`, `"duration_ms"`, and SpanView fields `start_time`, `parent_signal_id`, `status`, `message`.
- **TestQueryResponseShape** — Struct serialization test: confirms `"execution_time_ms"`, `"total"`, `"rows"` as array of objects (not arrays), no `"columns"` or `"row_count"` keys.
- **TestRulesNilPool** — Two subtests (GET + POST `/api/v1/rules`) confirming both return 503 `"database unavailable"` (not 501), proving DB-backed code path on both read and write operations.

### handler_apps_test.go (4 tests)

- **TestListAppsNilPool** — 503 + `"storage unavailable"` when pool is nil.
- **TestCreateAppNilPool** — Nil-pool guard fires before JSON parsing; confirms handler is not a stub.
- **TestGetAppKeyNilPool** — 503 on nil pool.
- **TestRotateAppKeyNilPool** — 503 on nil pool.

## Verification

```
go test ./internal/ingest/... -count=1
ok  github.com/argusxdr/argus/internal/ingest 6.092s
```

All 8 new tests pass. No regressions.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed pre-existing build failure in receiver_http_test.go**
- **Found during:** Task 1 (blocked test compilation)
- **Issue:** All `NewHTTPReceiver` calls in `receiver_http_test.go` were missing the `broadcaster *SignalBroadcaster` parameter added in a prior plan, causing the entire `ingest` test package to fail to compile.
- **Fix:** Added `nil` as the broadcaster argument in all 9 call sites.
- **Files modified:** `internal/ingest/receiver_http_test.go`
- **Commit:** 5922c9e

## Known Stubs

None. All handlers tested return real 503 degradation responses with correct error bodies.

## Self-Check: PASSED

- `internal/ingest/handler_response_test.go` — FOUND
- `internal/ingest/handler_apps_test.go` — FOUND
- Commit 5922c9e — FOUND
- Commit 6b939d8 — FOUND
- `go test ./internal/ingest/... -count=1` — PASSED (ok, 6.092s)
