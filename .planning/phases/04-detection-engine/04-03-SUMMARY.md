---
phase: 04-detection-engine
plan: "03"
subsystem: detection
tags: [postgres, pgx, detection-rules, hot-reload, polling, yaml]

requires:
  - phase: 04-detection-engine/04-01
    provides: RuleStore, Rule struct, engine package foundation

provides:
  - DBRuleLoader with MAX(version) polling (45s interval, configurable)
  - ParseRuleYAML exported helper (engine package)
  - TestRuleStore_ReplaceAll_Atomic concurrent reader test
  - internal/detection/loader package

affects:
  - 04-detection-engine/04-04
  - 04-detection-engine/04-05
  - cmd/argus (wiring DBRuleLoader at startup)

tech-stack:
  added: []
  patterns:
    - "querier interface on DBRuleLoader for testability without external mock library"
    - "MAX(version) short-circuit before fetching all rule YAML"
    - "ParseRuleYAML as shared parse core — file loader and DB loader both delegate to it"

key-files:
  created:
    - internal/detection/loader/db_loader.go
    - internal/detection/loader/db_loader_test.go
  modified:
    - internal/detection/engine/loader.go
    - internal/detection/engine/store_test.go

key-decisions:
  - "Use querier interface (not concrete pgxpool.Pool) on DBRuleLoader to enable in-process fake tests without pgxmock"
  - "ParseRuleYAML factored from LoadRuleFromFile; both paths share the same unmarshal+validate core"
  - "ReplaceAll takes []Rule (value types) matching existing RuleStore design, not []*Rule as plan suggested"

patterns-established:
  - "DB polling: MAX(version) check before fetching rows — avoids N-row query when nothing changed"
  - "Malformed YAML rows: log + skip, never abort the reload loop"

requirements-completed:
  - D-08
  - D-09
  - D-10
  - D-11

duration: 25min
completed: "2026-04-19"
---

# Phase 04 Plan 03: DB Rule Hot-Reload Summary

**DBRuleLoader polls detection_rules every 45s via MAX(version) short-circuit and hot-swaps the in-memory rule set atomically through RuleStore.ReplaceAll**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-04-19T09:35:00Z
- **Completed:** 2026-04-19T10:00:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- `ParseRuleYAML([]byte) (Rule, error)` exported from engine package — shared parse core for file and DB loaders
- `DBRuleLoader` created in new `internal/detection/loader` package; polls detection_rules using `SELECT COALESCE(MAX(version), 0)` and skips reload when version unchanged
- Concurrent atomic test (`TestRuleStore_ReplaceAll_Atomic`) proves 100 readers never see a partial rule set during `ReplaceAll`
- 4 test cases cover: interval default (0→45s), version bump triggers load, unchanged version skips ReplaceAll, malformed YAML rows skipped but valid rows loaded

## Task Commits

1. **Task 1: Confirm/add ReplaceAll + concurrent test** - `221bb3c` (test)
2. **Task 2: DBRuleLoader with MAX(version) polling** - `0da4e7e` (feat)

## Files Created/Modified

- `internal/detection/engine/loader.go` - Added `ParseRuleYAML` exported helper; `LoadRuleFromFile` now delegates to it
- `internal/detection/engine/store_test.go` - Added `TestRuleStore_ReplaceAll_Atomic` with 100 concurrent readers
- `internal/detection/loader/db_loader.go` - DBRuleLoader: `New`, `Run`, `reload`, `Interval`; uses `querier` interface for testability
- `internal/detection/loader/db_loader_test.go` - 4 tests using in-process fakeQuerier (no external mock dependency)

## Decisions Made

- Used a `querier` interface wrapping the two pgxpool methods actually needed (`QueryRow`, `Query`) so tests run without pgxmock (not in go.mod). `New` accepts `*pgxpool.Pool` (implements `querier`); internal constructor accepts the interface.
- `ReplaceAll` takes `[]Rule` (value types) to match the existing `RuleStore` API — plan mentioned `[]*Rule` but the store was never pointer-based.
- `lastVersion` initialized to `-1` so that a DB with `MAX(version)=0` still triggers the first load.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] fakeQuerier implemented instead of pgxmock**
- **Found during:** Task 2 (test authoring)
- **Issue:** pgxmock not in go.mod; importing it would require `go get` and a new dependency
- **Fix:** Created lightweight `fakeQuerier`/`fakeRow`/`fakeRows` types implementing only the two methods needed; covers all 4 test scenarios without external deps
- **Files modified:** internal/detection/loader/db_loader_test.go
- **Verification:** `go test ./internal/detection/loader/... -timeout 30s` passes
- **Committed in:** 0da4e7e (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — missing dependency)
**Impact on plan:** Equivalent test coverage with zero new external dependencies.

## Issues Encountered

- engine_test.go referenced `EvaluateTier1`/`EvaluateTier23` methods that were already present in engine.go (added by a prior session); no fix needed — package compiled cleanly.

## Next Phase Readiness

- DBRuleLoader ready to wire into `cmd/argus` startup (alongside YAML bootstrap loader)
- `ParseRuleYAML` available for any future component that parses rule YAML from non-file sources
- No blockers for 04-04 (detection pipeline integration)

---
*Phase: 04-detection-engine*
*Completed: 2026-04-19*
