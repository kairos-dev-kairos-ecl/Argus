---
phase: quick-260509-wcz
plan: 1
subsystem: storage
tags: [clickhouse, timestamp, bug-fix, datetime64]
dependency_graph:
  requires: []
  provides: [correct-signal-timestamps-in-clickhouse]
  affects: [signal-queries, trace-time-bucketing, retention-policies]
tech_stack:
  added: []
  patterns: [pass-time.Time-not-int64-for-DateTime64-columns]
key_files:
  created: []
  modified:
    - internal/storage/clickhouse.go
decisions:
  - "Pass time.Time (not int64) to batch.Append for all DateTime64 columns; clickhouse-go/v2 AppendRow unconditionally calls time.UnixMilli(v) for int64, so any other unit is silently corrupted"
metrics:
  duration: "< 5 minutes"
  completed: "2026-05-09"
  tasks_completed: 1
  files_changed: 1
---

# Phase quick-260509-wcz Plan 1: Fix ClickHouse Timestamp Corruption Summary

**One-liner:** Replaced `int64` (UnixNano / UnixMilli) with `time.Time` for the `timestamp` and `ingested_at` DateTime64 columns so clickhouse-go/v2 stores the correct year instead of year 2228+.

---

## What Was Done

The `batch.Append` call in `internal/storage/clickhouse.go` was passing `int64` values for two DateTime64 columns:

- `timestamp`: `sig.Timestamp.AsTime().UnixNano()` — nanoseconds treated as milliseconds by the driver → stored as year ~57 billion (visible as 2228 after overflow wrapping)
- `ingestedAt`: `time.Now().UnixMilli()` — milliseconds coincidentally within range but still the wrong pattern

Root cause (confirmed in driver source `clickhouse-go/v2@v2.24.0/lib/column/datetime64.go:221`):

```go
case int64:   col.col.Append(time.UnixMilli(v))  // always ms, ignores column precision
case time.Time: col.col.Append(v)                // correct: precision-aware
```

Fix: both variables now carry `time.Time` values.

---

## Changes

| File | Change |
|------|--------|
| `internal/storage/clickhouse.go` | `var timestamp int64` → `var timestamp time.Time`; `.AsTime().UnixNano()` → `.AsTime()`; `time.Now().UnixMilli()` → `time.Now()` |

Diff is exactly 3 lines removed / 6 lines added (comment + two type changes). No other files touched.

---

## Verification

```
go build ./internal/storage/...   # exit 0
go vet ./internal/storage/...     # exit 0
grep "var timestamp time.Time"    # 1 match at line 471
grep "ingestedAt := time.Now()"   # 1 match at line 479
grep "UnixNano\|UnixMilli"        # 0 matches
```

---

## Deviations from Plan

None — plan executed exactly as written.

---

## Commits

| Hash | Message |
|------|---------|
| ae0fb29 | fix(quick-260509-wcz): pass time.Time for DateTime64 columns to stop timestamp corruption |

---

## Self-Check: PASSED

- `internal/storage/clickhouse.go` — modified (confirmed by Edit tool)
- Commit ae0fb29 — confirmed by `git rev-parse --short HEAD`
- `go build ./internal/storage/...` — exit 0
- `go vet ./internal/storage/...` — exit 0
