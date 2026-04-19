---
phase: 04-detection-engine
plan: "04"
subsystem: detection-kairos
tags: [kairos, detection, fail-open, async, policy-evaluation]
dependency_graph:
  requires: ["04-01", "04-02", "04-03"]
  provides: ["conditional-kairos-evaluation", "kairos-fail-open", "kairos-timeout-handling"]
  affects: ["internal/detection/worker", "internal/ingest", "internal/kairos"]
tech_stack:
  added: []
  patterns: ["KairosEvaluator interface", "shouldCallKairos", "context.WithTimeout fail-open", "AlertWriter kairos param"]
key_files:
  created: []
  modified:
    - internal/kairos/evaluator_test.go
    - internal/detection/worker/async_worker.go
    - internal/detection/worker/async_worker_test.go
    - internal/ingest/alert_router.go
    - internal/ingest/alert_router_test.go
decisions:
  - "AlertWriter interface extended with *kairos.PolicyDecision param (nil = not called or failed)"
  - "KairosEvaluator interface defined in worker package for testability"
  - "shouldCallKairos checks RequiresKairos flag first, then sampling, then highRiskTagFn"
  - "Kairos timeout set to 500ms via context.WithTimeout; DeadlineExceeded increments counter"
  - "AlertRouter drops pipeline.AlertWriter assertion; now satisfies worker.AlertWriter"
metrics:
  duration: "~20 minutes"
  completed: "2026-04-19"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 5
---

# Phase 04 Plan 04: Kairos Evaluator Integration Summary

Wired Kairos policy evaluation into the async detection worker as a conditional, fail-open, timeout-tolerant sidecar call — never in the ingest hot path.

## What Was Built

### Task 1: Cache key fix and test (pre-existing code verified + test added)

The evaluator.go already had the correct cache key format (`rule_id + ":" + signal_id + ":" + trace_id`) and `EvaluationRequest.RuleID` already existed. The `RequiresKairos bool` field was already in `Rule`. Added the missing test:

- `TestEvaluator_CacheKeyIncludesRuleID` in `internal/kairos/evaluator_test.go`: invokes Evaluate twice with same signal+trace but different rule_ids, asserts both upstream HTTP calls happen (no cross-rule cache sharing — Pitfall 5 fix).

### Task 2: AsyncDetectionWorker Kairos integration

**KairosEvaluator interface** (async_worker.go):
```go
type KairosEvaluator interface {
    Evaluate(ctx context.Context, req *kairos.EvaluationRequest) (*kairos.PolicyDecision, error)
}
```
Nil value means Kairos is disabled — worker operates normally without it.

**shouldCallKairos logic**:
```go
func (w *AsyncDetectionWorker) shouldCallKairos(m engine.MatchResult, sig *v1.ArgusSignal) bool {
    if m.Rule.RequiresKairos { return true }  // D-15
    if w.rand() < w.samplingRate { return true } // 1-5% sampling window
    if w.highRiskTagFn != nil && w.highRiskTagFn(sig) { return true }
    return false
}
```

**Fail-open path** (D-13/D-14):
- `context.DeadlineExceeded` → increment `KairosTimeout` counter, return nil
- Any other error → log warn, return nil
- Never blocks alert creation

**AlertWriter signature change**:
```go
type AlertWriter interface {
    WriteAlert(ctx context.Context, m engine.MatchResult, kairosDecision *kairos.PolicyDecision) error
}
```

**AlertRouter update**: `WriteAlert` now accepts `*kairos.PolicyDecision`; persists it to the `kairos_decision` JSONB column (column already existed from migration 018). Nil decision writes SQL `null`.

### Tests Added (5 new Kairos tests)

| Test | Verifies |
|------|----------|
| `TestAsyncWorker_KairosCalledWhenRequiresKairos` | Rule flag true → Kairos invoked once, decision reaches writer |
| `TestAsyncWorker_KairosSkippedByDefault` | Flag false + rand=1.0 → Kairos not called, nil decision written |
| `TestAsyncWorker_KairosTimeoutFailOpen` | DeadlineExceeded → KairosTimeout+1, nil decision, no panic |
| `TestAsyncWorker_KairosErrorFailOpen` | Generic error → nil decision, no panic (D-13) |
| `TestAsyncWorker_KairosSamplingWindow` | rand=0.01 < rate=0.02 → invoked; rand=0.5 → skipped |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RequiresKairos, RuleID cache key, and BuildRequest already existed**
- Found during: Task 1 (pre-execution read)
- Issue: Prior work (phase 04-01/02) had already implemented these fields
- Fix: Verified correctness, added only the missing test
- Files modified: internal/kairos/evaluator_test.go only

**2. [Rule 1 - Bug] AlertRouter.WriteAlert pipeline interface assertion removed**
- Found during: Task 2 compilation
- Issue: `var _ pipeline.AlertWriter = (*AlertRouter)(nil)` would fail after signature change
- Fix: Removed the assertion; AlertRouter now satisfies worker.AlertWriter instead
- Files modified: internal/ingest/alert_router.go

## Known Stubs

None. Kairos decision is persisted to the `kairos_decision` JSONB column. The `highRiskTagFn` is optional and nil by default — callers can wire it via `WithHighRiskTagFn`.

## Self-Check: PASSED

Files exist:
- internal/detection/worker/async_worker.go — FOUND
- internal/detection/worker/async_worker_test.go — FOUND
- internal/ingest/alert_router.go — FOUND
- internal/kairos/evaluator_test.go — FOUND

Commits:
- 8e5fce0 — test(04-04): add TestEvaluator_CacheKeyIncludesRuleID
- 70be9bd — feat(04-04): wire conditional Kairos into AsyncDetectionWorker
