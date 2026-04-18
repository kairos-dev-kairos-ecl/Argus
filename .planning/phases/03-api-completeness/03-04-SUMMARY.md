---
phase: 03-api-completeness
plan: "04"
subsystem: api
tags: [health, observability, postgres, redis, clickhouse]
dependency_graph:
  requires: ["03-01"]
  provides: ["full-health-endpoint"]
  affects: ["monitoring", "operations"]
tech_stack:
  added: []
  patterns: ["per-component health checks", "graceful degradation reporting"]
key_files:
  modified:
    - cmd/argus/api.go
decisions:
  - "Health endpoint always returns 200 — callers check status field for degraded/healthy"
  - "nil component (not configured) reports unhealthy to trigger degraded overall status"
metrics:
  duration: "5 minutes"
  completed_date: "2026-04-18"
  tasks_completed: 1
  tasks_total: 1
  files_changed: 1
---

# Phase 3 Plan 04: Health Endpoint — All Three Backends Summary

**One-liner:** Health endpoint expanded to check ClickHouse, PostgreSQL, and Redis with per-component latency and structured JSON response.

## What Was Built

Updated `makeHealthHandler` in `cmd/argus/api.go` to accept `pgPool *pgxpool.Pool` and `redisClient *redis.Client` alongside the existing ClickHouse client. The handler now performs timed Ping checks against all three backends and returns a structured response:

```json
{
  "status": "healthy",
  "components": {
    "clickhouse": {"status": "healthy", "latency_ms": 2},
    "postgres":   {"status": "healthy", "latency_ms": 1},
    "redis":      {"status": "healthy", "latency_ms": 0}
  }
}
```

If any component is nil (not configured) or returns a ping error, its status is `"unhealthy"` and the overall status becomes `"degraded"`.

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Expand makeHealthHandler to check ClickHouse + PostgreSQL + Redis | a8b950f |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Self-Check: PASSED

- `cmd/argus/api.go` modified: confirmed
- Commit a8b950f exists: confirmed
- `go build ./cmd/argus/...` exits 0: confirmed
- `grep '"postgres"' cmd/argus/api.go`: matches
- `grep '"redis"' cmd/argus/api.go`: matches
- `grep "pgPool.Ping" cmd/argus/api.go`: matches
- `grep "redisClient.Ping" cmd/argus/api.go`: matches
