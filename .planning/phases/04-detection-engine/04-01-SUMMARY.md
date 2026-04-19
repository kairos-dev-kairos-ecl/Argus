---
phase: 04-detection-engine
plan: "01"
subsystem: detection-engine
tags: [detection, pipeline, migrations, tier-split, double-write-fix]
dependency_graph:
  requires: []
  provides:
    - EvaluateTier1 method on DetectionEngine (no-I/O inline path)
    - EvaluateTier23 method on DetectionEngine (async worker path)
    - Single-writer pipeline (WorkerPool is sole storage.Write caller)
    - Migration 018 (authoritative alerts DDL)
  affects:
    - internal/pipeline/detection.go (Tier1-only inline)
    - internal/detection/engine/engine.go (new tier-split API)
    - internal/storage/migrations/ (alerts table v2)
tech_stack:
  added: []
  patterns:
    - Tier split on DetectionEngine enables hot-path / async-path separation
    - signal.RelatedSignals used as proto-proxy for matched rule ID annotation
key_files:
  created:
    - internal/storage/migrations/018_create_alerts_v2.up.sql
    - internal/storage/migrations/018_create_alerts_v2.down.sql
  modified:
    - internal/detection/engine/engine.go
    - internal/detection/engine/engine_test.go
    - internal/pipeline/detection.go
    - internal/pipeline/detection_test.go
    - internal/pipeline/chain_test.go
    - internal/pipeline/router.go
decisions:
  - Router demoted to tombstone; WorkerPool is sole storage.Write caller to fix double-write
  - EvaluateTier1 is the inline-safe method (no Redis/PG I/O); EvaluateTier23 for async worker
  - signal.RelatedSignals used as proxy field for matched rule IDs until proto adds matched_rule_ids
  - Migration 018 uses DROP TABLE IF EXISTS CASCADE to supersede migrations 005 and 010
metrics:
  duration_minutes: 25
  completed_date: "2026-04-19"
  tasks_completed: 4
  tasks_total: 4
  files_created: 2
  files_modified: 6
---

# Phase 04 Plan 01: Wave-0 Prerequisites Summary

**One-liner:** Tier-split detection engine (EvaluateTier1/EvaluateTier23), double-write bug resolved via Router tombstone, and authoritative alerts migration 018 covering all AlertRouter columns.

---

## Tasks Completed

| # | Name | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Split DetectionEngine.Evaluate into EvaluateTier1 and EvaluateTier23 | 1def9c5 | engine.go, engine_test.go |
| 2 | Restrict DetectionProcessor to Tier 1, remove inline alert writes | 33a47ac | detection.go, detection_test.go |
| 3 | Fix double-write — remove Router from pipeline chain | 534ea91 | router.go, chain_test.go |
| 4 | Author migration 018 — authoritative alerts table | 65f1219 | 018_create_alerts_v2.up.sql, 018_create_alerts_v2.down.sql |

---

## What Was Built

### Engine Tier Split (Task 1)

`DetectionEngine` now exposes three entry points:

- `Evaluate()` — original, preserved for backward compatibility, evaluates all tiers
- `EvaluateTier1()` — filters to Tier==1 rules only; zero Redis/PG I/O; inline-safe
- `EvaluateTier23()` — evaluates Tier 2 (baseline) and Tier 3 (temporal/Redis); for async worker

### Inline Pipeline Restriction (Task 2)

`DetectionProcessor.Process()` now calls `EvaluateTier1` exclusively. Alert writes have been removed from the inline path. Tier 1 matches are annotated on the signal via `sig.RelatedSignals` with `"rule:<id>"` prefix entries.

### Double-Write Fix (Task 3)

`internal/pipeline/router.go` was demoted to a tombstone file. The `Router` struct and its `storage.Write` call have been removed. `WorkerPool.processSignal` is now the single writer per signal. `TestChain_NoDoubleWrite` confirms write count == 1.

### Alerts Migration 018 (Task 4)

`018_create_alerts_v2.up.sql` is the authoritative alerts DDL. It:
- Issues `DROP TABLE IF EXISTS alerts CASCADE` (supersedes 005 and 010)
- Covers all 22 columns referenced in `internal/ingest/alert_router.go` upsertAlert SQL
- Implements D-04 (`status CHECK`), D-05 (`fingerprint UNIQUE`), D-06 (`trace_id`, `signal_ids[]`), D-07 (`context JSONB`, `kairos_decision JSONB`)
- Creates 6 indexes for query performance

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Field] signal.RelatedSignals used as proxy for matched rule IDs**

- **Found during:** Task 2
- **Issue:** `ArgusSignal` has no `MatchedRuleIds` field in the proto; `Enrichment` struct also has no such field. Plan instructed to use `sig.MatchedRuleIds` guarded against nil.
- **Fix:** Used `sig.RelatedSignals` with `"rule:<id>"` prefix as a proto-proxy. Updated test assertions to match.
- **Files modified:** `internal/pipeline/detection.go`, `internal/pipeline/detection_test.go`
- **Impact:** Low — RelatedSignals is semantically adjacent; a future plan should add `matched_rule_ids []string` to the proto Enrichment message and migrate this annotation.

**2. [Rule 1 - Bug] TestDetectionProcessorWritesAlertOnMatch expected alert write**

- **Found during:** Task 2
- **Issue:** Existing test asserted `w.written == 1`, which contradicts the new no-inline-alert-write behavior.
- **Fix:** Updated the test to assert `w.written == 0` and verify `RelatedSignals` contains the rule tag.
- **Files modified:** `internal/pipeline/detection_test.go`
- **Commit:** 33a47ac

---

## Known Stubs

None. All plan goals are fully wired.

---

## Self-Check: PASSED

- `internal/detection/engine/engine.go` contains `func (e *DetectionEngine) EvaluateTier1` — FOUND
- `internal/detection/engine/engine.go` contains `func (e *DetectionEngine) EvaluateTier23` — FOUND
- `internal/detection/engine/engine_test.go` contains `TestEvaluate_TierSplit` — FOUND
- `internal/pipeline/detection.go` contains `EvaluateTier1` — FOUND
- `internal/pipeline/detection.go` has no `WriteAlert` or `EvaluateTier23` — CONFIRMED
- `internal/pipeline/chain_test.go` contains `TestChain_NoDoubleWrite` — FOUND
- `internal/storage/migrations/018_create_alerts_v2.up.sql` — FOUND
- `internal/storage/migrations/018_create_alerts_v2.down.sql` — FOUND
- Commits 1def9c5, 33a47ac, 534ea91, 65f1219 — all present
- `go build ./...` exits 0 — CONFIRMED
- `go test ./internal/detection/engine/... ./internal/pipeline/...` exits 0 — CONFIRMED
