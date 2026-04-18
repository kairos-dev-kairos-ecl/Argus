---
phase: 03-api-completeness
plan: "03"
subsystem: ingest-api
tags: [rules, apps, postgresql, api, crud]
dependency_graph:
  requires: [03-01]
  provides: [rules-crud-db-backed, apps-crud-db-backed]
  affects: [frontend-rules-page, frontend-apps-page]
tech_stack:
  added: []
  patterns: [pgx-v5-inline-handlers, sha256-api-keys, nil-pool-503-guard]
key_files:
  created:
    - internal/ingest/handler_apps.go
  modified:
    - internal/ingest/handler_stubs.go
    - internal/ingest/receiver_query.go
    - cmd/argus/api.go
decisions:
  - "Inline pgx/v5 queries in each handler (no delegation to in-memory Serve* handlers) to ensure rules persist across server restarts"
  - "RuleStore sync uses Remove+Add (no Update method exists) after DB success for live detection"
  - "API keys use arg_ prefix + 32 random bytes hex, SHA256 hashed for storage, plaintext returned once only"
metrics:
  duration: "12 minutes"
  completed_date: "2026-04-18"
  tasks_completed: 2
  files_changed: 4
---

# Phase 03 Plan 03: Rules + Apps DB-Backed Handlers Summary

DB-backed rules CRUD against detection_rules PostgreSQL table and apps CRUD with crypto/rand API key generation, replacing all 501 stubs.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Replace rules stubs with inline DB-backed implementations + add pool to QueryHandler | 083032e | handler_stubs.go, receiver_query.go, api.go |
| 2 | Implement apps CRUD handlers | 606f676 | handler_apps.go |

## What Was Built

**Rules handlers (handler_stubs.go):** All 5 rule handlers (list, create, get, update, delete) now query the `detection_rules` PostgreSQL table directly via `h.pool` (pgx/v5). Each handler has a nil-pool guard returning 503. After DB success, the in-memory `h.store` is synced (Remove+Add for update, since no Update method exists on RuleStore) so live detection sees changes immediately. handleValidateRule and handleTestRule don't need DB access.

**QueryHandler pool field (receiver_query.go):** Added `pool *pgxpool.Pool` field and `SetPool(p *pgxpool.Pool)` method. Import added for `github.com/jackc/pgx/v5/pgxpool`.

**Pool wiring (api.go):** `queryHandler.SetPool(pgPool)` called after `SetAlertRouter` when pgPool is non-nil.

**Apps handlers (handler_apps.go):** New file with handleListApps, handleCreateApp, handleGetAppKey, handleRotateAppKey. API keys are `arg_` + 64 hex chars (32 random bytes), with first 12 chars as prefix, SHA256 hash stored in DB. Plaintext returned once at creation/rotation.

## Acceptance Criteria Verified

- `grep -c "detection_rules" internal/ingest/handler_stubs.go` → 6 (DB queries present)
- `grep -c "SetPool" internal/ingest/receiver_query.go` → 2 (declaration + method)
- `grep -c "SetPool" cmd/argus/api.go` → 1
- No Serve* delegation in handler_stubs.go (only comment)
- `grep -c "StatusNotImplemented" internal/ingest/handler_stubs.go` → 0
- `grep -c "database unavailable" internal/ingest/handler_stubs.go` → 5 (all 5 rule handlers guarded)
- `grep -cE "h\.pool\.(Query|Exec|QueryRow)" internal/ingest/handler_stubs.go` → 5
- `go build ./internal/ingest/...` → 0
- `go build ./...` → 0

## Deviations from Plan

**1. [Rule 1 - Bug] jsonError already declared in handler_alerts.go**
- **Found during:** Task 1 first build attempt
- **Issue:** handler_stubs.go declared `jsonError` but it already existed in handler_alerts.go in the same package
- **Fix:** Removed jsonError declaration from handler_stubs.go; existing function in handler_alerts.go used
- **Commit:** 083032e (fixed before commit)

## Known Stubs

None — all rules and apps handlers are fully DB-backed. handleTestRule returns an empty test result (intentional stub for future implementation, does not block the plan's goal of removing 501s).
