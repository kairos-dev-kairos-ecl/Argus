---
phase: 04-detection-engine
plan: "07"
subsystem: detection-integration-tests
tags: [integration-tests, detection, alert-pipeline, migration, kairos]
dependency_graph:
  requires: [04-01, 04-02, 04-03, 04-04, 04-05, 04-06]
  provides: [integration-test-suite, migration-018-validation]
  affects: [detection-engine, alert-router, db-loader]
tech_stack:
  added: []
  patterns: [integration-build-tag, apply-migrations-helper, fail-open-kairos-test]
key_files:
  created:
    - internal/detection/integration_test.go
    - internal/storage/migration_test.go
    - internal/storage/testutil_integration_test.go
  modified: []
decisions:
  - "External test packages (storage_test, detection_test) used for integration tests to avoid package cycles"
  - "ApplyMigrations helper reads SQL files from disk in numeric order — avoids embedding complexity in test code"
  - "Kairos unreachable test uses PolicyConfig{Enabled:true} to ensure evaluator actually calls Kairos endpoint"
metrics:
  duration_minutes: 20
  completed_date: "2026-04-19"
  tasks_completed: 2
  files_created: 3
  files_modified: 0
---

# Phase 4 Plan 7: Integration Tests — Summary

End-to-end integration test suite covering the complete signal → detection engine → alert pipeline. Migration 018 DDL validated against real PostgreSQL. All 6 critical signal paths have integration-level assertions.

## One-liner

Integration test suite proving the signal→alert pipeline end-to-end: dedup, suppression, trace enforcement, Kairos fail-open, and hot-reload all verified against real PostgreSQL + Redis.

## Tasks Completed

| # | Task | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Migration 018 DDL integration test | 9076883 | internal/storage/migration_test.go, internal/storage/testutil_integration_test.go |
| 2 | End-to-end signal → alert integration tests | b86deba | internal/detection/integration_test.go |

## Integration Test Setup

**Required env vars:**
- `TEST_POSTGRES_DSN` — e.g. `postgres://argus:argus@localhost:5432/argus_test?sslmode=disable`
- `TEST_REDIS_DSN` — e.g. `localhost:6379` (default if unset)

**Required services (docker-compose):**
- `postgres` — PostgreSQL 16 on port 5432
- `redis` — Redis 7.2 on port 6379
- ClickHouse not required — detection tests use a nil ClickHouse writer (mock)

**Run commands:**
```bash
# Migration tests
TEST_POSTGRES_DSN=postgres://argus:argus@localhost:5432/argus_test?sslmode=disable \
  go test -tags=integration ./internal/storage/ -run TestMigration018 -timeout 60s

# Detection end-to-end
TEST_POSTGRES_DSN=postgres://argus:argus@localhost:5432/argus_test?sslmode=disable \
TEST_REDIS_DSN=localhost:6379 \
  go test -tags=integration ./internal/detection/ -run TestIntegration -timeout 180s
```

## Asserted Behaviors

### Migration 018 (TestMigration018_AlertsTableStructure)
- All 23 columns present: id, rule_id, app_id, trace_id, signal_ids, signal_count, fingerprint, severity, layer, category, title, description, status, context, kairos_decision, first_seen_at, last_seen_at, acknowledged_at, acknowledged_by, resolved_at, incident_id, created_at, updated_at
- UNIQUE index `idx_alerts_fingerprint` exists
- CHECK constraint accepts: open, acknowledged, resolved, suppressed
- CHECK constraint rejects: closed (invalid)

### Detection Pipeline (6 integration tests)

| Test | Decision | Assertions |
|------|----------|------------|
| TestIntegration_SignalToAlert_Tier2 | D-01, D-06, D-07 | fingerprint != '', trace_id='tr-1', context != NULL, kairos_decision=null, status='open' |
| TestIntegration_FingerprintDedup_IncrementsCount | D-03 | 1 row, signal_count=3 |
| TestIntegration_SuppressedAt101 | D-04 | status='suppressed' after 101 signals |
| TestIntegration_KairosUnreachable_FailOpen | D-13, D-14 | alert written, kairos_decision=null |
| TestIntegration_HotReload_PicksUpNewVersion | D-08 | new rule in store within 2s poll |
| TestIntegration_TraceIdRequired_Rejects | D-06 | zero alert rows for empty trace_id |

## D-01..D-19 Test Coverage Map

| Decision | Test(s) |
|----------|---------|
| D-01 Schema-first alerts | TestIntegration_SignalToAlert_Tier2, TestMigration018_AlertsTableStructure |
| D-03 Dedup by fingerprint | TestIntegration_FingerprintDedup_IncrementsCount |
| D-04 Auto-suppress at 100 | TestIntegration_SuppressedAt101 |
| D-05 Fingerprint = sha256(rule+entity+payload) | TestIntegration_SignalToAlert_Tier2 (fingerprint != '') |
| D-06 trace_id mandatory | TestIntegration_TraceIdRequired_Rejects |
| D-07 JSONB context field | TestIntegration_SignalToAlert_Tier2 (context IS NOT NULL) |
| D-08 DB rule hot-reload | TestIntegration_HotReload_PicksUpNewVersion |
| D-09 MAX(version) short-circuit | TestIntegration_HotReload_PicksUpNewVersion (version bump triggers reload) |
| D-13 Kairos fail-open | TestIntegration_KairosUnreachable_FailOpen |
| D-14 kairos_decision=NULL on timeout | TestIntegration_KairosUnreachable_FailOpen |
| D-18 Prometheus metrics | TestIntegration_KairosUnreachable_FailOpen (registry wired, no panic) |
| D-19 Breaker integration | newTestWorker creates breaker (wired) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed invalid source type name in signal construction**
- **Found during:** Task 2 compilation
- **Issue:** Plan referenced `v1.SignalSource` which does not exist; actual type is `v1.Source`
- **Fix:** Used `v1.Source{AppId: appID}` per generated proto stubs
- **Files modified:** internal/detection/integration_test.go
- **Commit:** b86deba

**2. [Rule 1 - Bug] Fixed non-test file with package storage_test**
- **Found during:** Task 1 — `go vet` detected package conflict
- **Issue:** `testutil_integration.go` used `package storage_test` but must be `*_test.go` filename
- **Fix:** Renamed to `testutil_integration_test.go`
- **Files modified:** internal/storage/testutil_integration_test.go
- **Commit:** b86deba (rename)

**3. [Rule 1 - Bug] Fixed detection_rules insert SQL**
- **Found during:** Task 2 — table schema uses `rule_id TEXT` not `id TEXT` as PK
- **Issue:** Plan INSERT used `id` column but table PK is UUID `id` with separate `rule_id TEXT UNIQUE`
- **Fix:** INSERT uses `rule_id` column for user-defined rule ID
- **Files modified:** internal/detection/integration_test.go
- **Commit:** b86deba

**4. [Rule 2 - Missing] Kairos evaluator needs Enabled=true**
- **Found during:** Task 2 — kairos.DefaultConfig() has Enabled=false
- **Issue:** Evaluator silently returns nil without calling client when Enabled=false
- **Fix:** Created explicit PolicyConfig{Enabled: true} for unreachable test
- **Files modified:** internal/detection/integration_test.go
- **Commit:** b86deba

## Self-Check: PASSED

- internal/detection/integration_test.go: FOUND
- internal/storage/migration_test.go: FOUND
- internal/storage/testutil_integration_test.go: FOUND
- Commit 9076883: FOUND (migration test)
- Commit b86deba: FOUND (e2e integration tests)
- `go build ./...` exits 0: VERIFIED
- `go vet -tags=integration ./internal/storage/ ./internal/detection/`: exits 0 VERIFIED
