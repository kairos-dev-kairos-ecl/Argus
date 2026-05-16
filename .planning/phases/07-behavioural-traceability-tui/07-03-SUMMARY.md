---
phase: 07-behavioural-traceability-tui
plan: "03"
subsystem: baseline
tags: [baseline, session, drift, levenshtein, clickhouse, postgresql, redis, tdd]
dependency_graph:
  requires: [07-01, migrations/011_session_baseline.up.sql]
  provides: [SessionBaselineEngine, SessionProfileStore, ComputeSessionDrift]
  affects: [internal/baseline, future-07-05-behaviour-endpoint]
tech_stack:
  added: []
  patterns:
    - dual-write Redis (best-effort 30m TTL) + PostgreSQL (authoritative UPSERT ON CONFLICT)
    - async 10-min compute ticker (mirrors BaselineEngine pattern exactly)
    - two-row DP Levenshtein on []int32 (O(m+n) space)
    - TDD red-green on all three tasks
key_files:
  created:
    - internal/baseline/session_profile.go
    - internal/baseline/session_store.go
    - internal/baseline/drift.go
    - internal/baseline/drift_test.go
    - internal/baseline/session_engine.go
    - internal/baseline/session_engine_test.go
  modified: []
decisions:
  - "Named sessionAgg package type instead of anonymous struct so buildSessionProfile is testable without a real ClickHouse connection"
  - "sessionPercentile helper avoids import of math/sort alternatives; uses index-based percentile matching existing BaselineEngine style"
  - "ComputeSessionDrift returns 1.0 (not 0.0) when one slice is empty — empty vs non-empty is maximum divergence, not zero drift"
metrics:
  duration_seconds: 172
  completed_date: "2026-05-16"
  tasks_completed: 3
  tasks_total: 3
  files_created: 6
  files_modified: 0
---

# Phase 07 Plan 03: Session Baseline Engine & Drift — Summary

**One-liner:** Session baseline engine with dual-write Redis/PostgreSQL store and pure normalised Levenshtein drift function, mirroring existing BaselineEngine pattern exactly.

---

## Objective

Extend `internal/baseline/` with `SessionBaselineEngine`: async 10-min compute cadence matching the signal-level engine, per-app_id profiles, dual-write Redis (30m TTL) + PostgreSQL (durable UPSERT), and a pure `ComputeSessionDrift` function using normalised Levenshtein distance on layer enum sequences. Required by REQ-P7-08 (`GET /api/v1/conversations/{id}/behaviour`) drift score calculation.

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | SessionProfile types + SessionProfileStore (dual-write) | 432d351 | session_profile.go, session_store.go |
| 2 | ComputeSessionDrift (normalised Levenshtein) | 68c0b0b | drift.go, drift_test.go |
| 3 | SessionBaselineEngine compute loop | f4bec50 | session_engine.go, session_engine_test.go |

---

## Verification Results

```
go build ./internal/baseline/...         exit 0
go test ./internal/baseline/... -run "TestComputeSessionDrift|TestBuildSessionProfile"   exit 0

=== PASS: TestComputeSessionDrift (10 cases) ===
  both_empty → 0.0
  identical  → 0.0
  one_empty  → 1.0
  substitution [1,2,3] vs [1,2,4] → 0.333...
  deletion [1,2,3,4] vs [1,2,3]   → 0.25
  completely_different             → 1.0
  result_never_exceeds_1.0         PASS
  single_element_{identical,different} PASS

=== PASS: TestBuildSessionProfile_Modal ===
  modal([3,5,7]×2 + [3,7]×1) == [3,5,7]
  SampleCount == 3
  AnomalyRate == 3/23
  P50 == 150ms, P95 == 150ms

=== PASS: TestBuildSessionProfile_Empty ===
=== PASS: TestDefaultSessionBaselineConfig ===
```

---

## Architecture Decisions

**Named `sessionAgg` type (not anonymous struct):** The plan noted that `buildSessionProfile` with an anonymous struct parameter is untestable from the test file. Promoted to a named package-level type `sessionAgg` so `session_engine_test.go` can call `buildSessionProfile` directly without any ClickHouse fixture.

**`ComputeSessionDrift` empty-vs-non-empty = 1.0:** When one sequence is empty and the other isn't, this is maximum divergence (the session activated zero layers vs N layers). Returning 1.0 is semantically correct and matches the normalised Levenshtein definition (edit distance = max(0, N) / N = 1.0).

**`sessionPercentile` index-based:** Uses `int(float64(len-1) * p)` — same approach as the plan spec. For small session counts (typical for per-app in 24h), this is accurate enough and avoids interpolation complexity.

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Anonymous struct in `buildSessionProfile` is untestable**
- **Found during:** Task 3 implementation
- **Issue:** Plan showed `buildSessionProfile` accepting `[]struct{ layers []int32; durMS float64; ... }` — anonymous struct parameters cannot be constructed from external callers (the test file would need identical struct literal syntax with no named type to reference)
- **Fix:** Promoted to named `sessionAgg` type at package level; test can now construct `[]sessionAgg{...}` directly
- **Files modified:** session_engine.go, session_engine_test.go
- **Impact:** Purely internal — no exported API changed

---

## Known Stubs

None. All four files contain real implementations. The `LayerDwellMS` field is initialised to `map[string]float64{}` (empty, not nil) — per-layer dwell time computation is deferred to a future plan when per-layer timing data is available in the signals table; the field schema is wired and ready.

---

## Self-Check: PASSED

Files exist:
- FOUND: internal/baseline/session_profile.go
- FOUND: internal/baseline/session_store.go
- FOUND: internal/baseline/drift.go
- FOUND: internal/baseline/drift_test.go
- FOUND: internal/baseline/session_engine.go
- FOUND: internal/baseline/session_engine_test.go

Commits exist:
- FOUND: 432d351 (SessionProfile + SessionProfileStore)
- FOUND: 68c0b0b (ComputeSessionDrift)
- FOUND: f4bec50 (SessionBaselineEngine)
