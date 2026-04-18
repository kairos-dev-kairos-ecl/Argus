---
phase: 03-api-completeness
plan: 02
subsystem: backend-api
tags: [api, response-shapes, typescript-alignment, layer-status, trace, query]
one_liner: "Fixed three endpoint response shapes (layer status, trace, query) to match frontend TypeScript interfaces exactly"
dependency_graph:
  requires: [03-01]
  provides: [layer-status-json, trace-spans-json, query-rows-json]
  affects: [web/src/hooks/useQuery.ts, web/src/hooks/useTrace.ts, web/src/hooks/useReasoningGraph.ts]
tech_stack:
  added: []
  patterns: [map[string]interface{} for flexible row scanning, pointer-to-string for nullable timestamps]
key_files:
  modified:
    - internal/ingest/receiver_query.go
decisions:
  - "Used yellow status when layer had prior signals but none in last 5 min, gray when never seen"
  - "Detection slice returns empty []Detection{} — will be populated in Phase 4"
  - "SpanView.Message maps to signal.Category as the closest human-readable label"
metrics:
  duration_minutes: 5
  completed_date: "2026-04-18"
  tasks_completed: 3
  files_modified: 1
requirements_satisfied: [REQ-P3-02, REQ-P3-03, REQ-P3-04]
---

# Phase 3 Plan 02: API Response Shape Alignment Summary

Fixed three endpoints in `internal/ingest/receiver_query.go` so their JSON responses match the frontend TypeScript interfaces in `web/src/types/index.ts` exactly.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix layer status response shape | 067ae13 | internal/ingest/receiver_query.go |
| 2 | Fix trace response shape (signals[] to spans[]) | 067ae13 | internal/ingest/receiver_query.go |
| 3 | Fix query response shape (arrays to objects + execution_time_ms) | 067ae13 | internal/ingest/receiver_query.go |

## What Changed

### Task 1: Layer Status

Before:
```json
{"layers": [{"layer": 1, "name": "Hardware", "signal_count": 0, "status": "idle"}]}
```

After:
```json
{"layers": [{"layer": "L1_HARDWARE", "status": "gray", "last_signal_time": null, "signal_count_5min": 0}]}
```

- `layer` changed from int32 to string enum (L1_HARDWARE through L10_APPLICATION)
- `status` changed from active/idle/unknown to green/yellow/gray
- `signal_count` renamed to `signal_count_5min`
- `last_seen_at` renamed to `last_signal_time` and is now nullable pointer
- `name` field removed entirely
- `layerNames` map replaced by `layerEnumStrings` map

### Task 2: Trace Response

Before:
```json
{"trace_id": "...", "signals": [...raw proto objects...]}
```

After:
```json
{"trace_id": "...", "spans": [{"signal_id": "...", "layer": "L4_TRANSFORMER", "start_time": "...", "duration_ms": 12.5, "status": "ok", "message": "inference"}], "detections": [], "duration_ms": 450}
```

- Added `SpanView` struct mapping signal fields to span shape
- Added `Detection` struct (empty for now, Phase 4 will populate)
- `duration_ms` computed from min/max signal timestamps
- Signal severity >= 4 maps to status "error", otherwise "ok"

### Task 3: Query Response

Before:
```json
{"columns": ["col1", "col2"], "rows": [[v1, v2]], "row_count": 1}
```

After:
```json
{"rows": [{"col1": v1, "col2": v2}], "total": 1, "execution_time_ms": 3}
```

- Rows changed from `[][]interface{}` to `[]map[string]interface{}`
- `execution_time_ms` added, measured from request validation to scan completion
- `total` replaces `row_count`, `columns` field removed

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

- `Detections: []Detection{}` in TraceResponse — intentionally empty. Phase 4 will wire the detection engine query.

## Self-Check: PASSED

- internal/ingest/receiver_query.go: FOUND
- commit 067ae13: FOUND
