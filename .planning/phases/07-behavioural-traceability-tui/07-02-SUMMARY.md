---
phase: 07-behavioural-traceability-tui
plan: 02
subsystem: internal/trace
tags: [trace, reconstruction, dag, timeline, clickhouse, query-time]
dependency_graph:
  requires: [07-01]
  provides: [internal/trace RunReconstructor, internal/trace TimelineBuilder]
  affects: [07-04-http-endpoints, 07-05-tui]
tech_stack:
  added: []
  patterns:
    - sql.NullFloat64/sql.NullInt64/sql.NullString scan pattern for Nullable ClickHouse columns
    - In-memory graph assembly from parent_span_id adjacency
    - First-seen layer activation sequence derivation
key_files:
  created:
    - internal/trace/graph.go
    - internal/trace/query.go
    - internal/trace/timeline.go
    - internal/trace/builder.go
    - internal/trace/partial.go
    - internal/trace/timeline_builder.go
    - internal/trace/builder_test.go
    - internal/trace/timeline_builder_test.go
  modified: []
decisions:
  - "Used sql.NullFloat64/NullInt64/NullString (database/sql) for Nullable columns, consistent with existing codebase pattern in internal/ingest/receiver_query.go"
  - "scanRunNode / scanTimelineEvent share identical column order matching SelectRunSignalsSQL column list — no positional ambiguity"
  - "assembleGraph and assembleTimeline are package-private pure functions (no DB) — directly testable without mocks"
  - "attachOrphans: when no roots exist, the earliest-timestamped orphan is promoted as the implicit attachment anchor; it is flagged IsOrphan=true but receives no self-edge"
metrics:
  duration: "4 minutes"
  completed_date: "2026-05-16"
  tasks_completed: 3
  files_created: 8
  tests_added: 4
---

# Phase 7 Plan 2: Wave 1 Reconstruction Layer — internal/trace/ Package Summary

**One-liner:** Pure query-time DAG reconstruction from span_id/parent_span_id adjacency (RunReconstructor) and first-seen layer activation sequence from session_id/conversation_id (TimelineBuilder), with orphan detection and 35-column ClickHouse scan.

---

## What Was Built

### internal/trace/graph.go
Declares the core type hierarchy:
- `RunGraph` — root container: `Meta RunMeta`, `Nodes []*RunNode`, `Edges []RunEdge`
- `RunNode` — signal with causality fields: `SpanID`, `ParentSpanID`, `IsOrphan`, `IsAnomaly`, `CtxSummary`
- `RunEdge` — typed directed edge: `EdgeParentChild` or `EdgeTemporal`
- `RunMeta` — trace-level aggregates: `PeakDeviation`, `LayersPresent`, `OrphanCount`
- `AnomalyDeviationThreshold float32 = 2.0`

### internal/trace/query.go
Three SQL constant strings, all with fixed column order matching the scan functions:
- `SelectRunSignalsSQL` — `WHERE trace_id = ? ORDER BY timestamp ASC`
- `SelectSessionSignalsSQLBySessionID` — `WHERE session_id = ? ORDER BY timestamp ASC`
- `SelectSessionSignalsSQLByConversationID` — `WHERE conversation_id = ? ORDER BY timestamp ASC`

All 35 columns selected: identity + classification + temporal + 30 diagnostic ctx_* fields (L3–L10).

### internal/trace/timeline.go
- `SessionTimeline` — scoped timeline: `ScopeKind`, `ScopeID`, `Events`, `ByLayer`, `Aggregates`
- `TimelineEvent` — signal with `LayerLabel string` (format: "L{N}") and `IsAnomaly bool`
- `SessionAggregates` — `LayerActivationSequence []int32`, `PeakDeviation`, `AnomalyCount`, `TotalSignals`, `DurationMS`

### internal/trace/builder.go
`RunReconstructor` — query-time component:
- `NewRunReconstructor(ch driver.Conn) *RunReconstructor`
- `BuildRun(ctx, traceID) (*RunGraph, error)` — returns `ErrTraceNotFound` when zero signals match
- `scanRunNode(rows)` — scans 35 columns; CtxSummary populated only with non-null ctx fields
- `assembleGraph(traceID, nodes)` — walks parent_span_id, builds parent_child edges, detects orphans, computes meta

### internal/trace/partial.go
`attachOrphans(roots, orphans, edges)`:
- Marks each orphan `IsOrphan=true`
- Attaches each to first root via `EdgeTemporal`
- When no roots: promotes earliest-timestamped orphan as anchor

### internal/trace/timeline_builder.go
`TimelineBuilder` — query-time component:
- `NewTimelineBuilder(ch driver.Conn) *TimelineBuilder`
- `BuildFromSession(ctx, sessionID) (*SessionTimeline, error)`
- `BuildFromConversation(ctx, conversationID) (*SessionTimeline, error)`
- `assembleTimeline(scopeKind, scopeID, events)` — pure function, derives `LayerActivationSequence` in first-seen order

---

## Tests

| Test | File | Covers |
|------|------|--------|
| `TestTypesCompile` | builder_test.go | All exported types and constants compile |
| `TestAssembleGraph` | builder_test.go | 3-node graph: 1 parent_child edge, orphan=true, PeakDeviation=2.5, LayersPresent=[3,5,7] |
| `TestAssembleTimeline_LayerActivation` | timeline_builder_test.go | LayerActivationSequence=[3,5,7] (dedup, first-seen), TotalSignals=4, PeakDeviation=2.5, DurationMS=3000 |
| `TestAssembleTimeline_Empty` | timeline_builder_test.go | Empty events returns valid zero-value SessionTimeline |

All tests: `go test ./internal/trace/... exits 0`

---

## Deviations from Plan

None — plan executed exactly as written.

The builder_test.go stub described in Task 1 was written with the full TestAssembleGraph test (which was specified in Task 2) because the test exercises assembleGraph which is in builder.go — separating them by task would have split a naturally cohesive test. The plan's Task 2 acceptance criteria (`func TestAssembleGraph`) are fully satisfied.

---

## Known Stubs

None. All functions are fully implemented. The `scanRunNode` and `scanTimelineEvent` scan implementations were marked as pseudocode in the plan (`/* implement: ... */`) but are fully coded with all 35 column scans and CtxSummary construction.

---

## Self-Check: PASSED

Files present:
- internal/trace/graph.go — FOUND
- internal/trace/query.go — FOUND
- internal/trace/timeline.go — FOUND
- internal/trace/builder.go — FOUND
- internal/trace/partial.go — FOUND
- internal/trace/timeline_builder.go — FOUND
- internal/trace/builder_test.go — FOUND
- internal/trace/timeline_builder_test.go — FOUND

Commits present:
- 3ebf2f7 feat(07-02): define RunGraph/RunNode/RunEdge/RunMeta types and SQL constants — FOUND
- fa1e17b feat(07-02): RunReconstructor with graph assembly and orphan detection — FOUND
- 7ffdcda feat(07-02): TimelineBuilder for session and conversation scopes — FOUND

Build: `go build ./internal/trace/...` exits 0
Tests: `go test ./internal/trace/...` exits 0 (4 tests pass)
