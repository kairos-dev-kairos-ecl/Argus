---
phase: 04-detection-engine
plan: 02
subsystem: detection-infrastructure
tags: [circuit-breaker, async-worker, prometheus, drop-policy, detection]
dependency_graph:
  requires: [04-01]
  provides: [04-03, 04-04, 04-05]
  affects: [internal/detection/worker, internal/detection/breaker, internal/metrics]
tech_stack:
  added: []
  patterns: [injectable-clock-testing, severity-based-drop, circuit-breaker-sampling]
key_files:
  created:
    - internal/detection/breaker/breaker.go
    - internal/detection/breaker/breaker_test.go
    - internal/detection/worker/async_worker.go
    - internal/detection/worker/async_worker_test.go
    - internal/detection/worker/drop_policy.go
    - internal/detection/worker/drop_policy_test.go
    - internal/metrics/detection.go
    - internal/metrics/detection_test.go
  modified: []
decisions:
  - "Severity enum offset: proto uses INFO=1, LOW=2 (not LOW=1 as plan assumed). SeverityInt maps accordingly."
  - "AlertWriter interface uses WriteAlert(ctx, MatchResult) — no signal parameter (matches pipeline.AlertWriter exactly)"
  - "Evaluator interface defined in worker package to enable test stubs without coupling to concrete engine type"
  - "INFO severity treated same as LOW for drop policy (sevInt <= 2 drops)"
metrics:
  duration: "~15 minutes"
  completed: "2026-04-19"
  tasks_completed: 3
  files_created: 8
---

# Phase 04 Plan 02: Async Detection Infrastructure Summary

Bounded async detection worker with circuit breaker and Prometheus metrics — 3 production-ready packages that compile and pass unit tests, ready for Wave 2/3 wiring.

## What Was Built

### Task 1: Circuit Breaker (D-19)

**Package:** `internal/detection/breaker`

**Constructor:** `New(halfOpenAfter time.Duration) *CircuitBreaker`

**State machine:**
- `StateClosed` (default) — all traffic passes
- `StateOpen` — tripped state, blocks or samples traffic
- `StateHalfOpen` — probe mode after 30s cooldown, allows traffic

**Trip thresholds:** `Evaluate(queueDepth, maxDepth int, p99Ms, maxP99Ms float64)` trips when `queueDepth > 80% * maxDepth` OR `p99Ms > maxP99Ms`

**Injectable clock:** `b.now func() time.Time` enables deterministic testing without `time.Sleep`

**Tests:** 6 — StartsClosed, TripsOnHighQueue, TripsOnHighP99, HalfOpenAfterCooldown, SucceedCloses, FailReopens

**Commit:** `319d04f`

---

### Task 2: Detection Prometheus Metrics (D-18)

**Package:** `internal/metrics`

**Constructor:** `NewDetection(reg prometheus.Registerer) *Detection`

**Exact metric names (D-18 spec):**

| Field | Metric Name | Type | Labels |
|-------|-------------|------|--------|
| `Latency` | `argus_detection_latency_seconds` | HistogramVec | `tier` |
| `QueueDepth` | `argus_detection_queue_depth` | Gauge | — |
| `Evaluations` | `argus_rule_evaluations_total` | CounterVec | `tier` |
| `AlertsCreated` | `argus_alert_created_total` | CounterVec | `severity`, `rule_id` |
| `Dropped` | `argus_detection_dropped_total` | CounterVec | `severity` |
| `KairosTimeout` | `kairos_timeout_total` | Counter | — |

Latency histogram buckets: `{.005, .01, .025, .05, .1, .25, .5, 1.0}`

**Tests:** 2 — AllMetricsRegistered, DropLabels

**Commit:** `476c9b6`

---

### Task 3: AsyncDetectionWorker (D-12, D-13, D-14, D-16, D-17, D-19)

**Package:** `internal/detection/worker`

**Constructor:**
```go
func New(
    eng Evaluator,
    aw AlertWriter,
    br *breaker.CircuitBreaker,
    m *metrics.Detection,
    log *zap.Logger,
    queueSize, workerCount int,
) *AsyncDetectionWorker
```
- `queueSize` defaults to 10,000 if <= 0 (D-16)
- `workerCount` defaults to `GOMAXPROCS * 2` if <= 0

**Drop policy (D-16, D-17):**

| Severity | Queue Full Behavior |
|----------|---------------------|
| INFO (1), LOW (2) | Always dropped, metric incremented |
| MEDIUM (3) | 50% probability drop |
| HIGH (4), CRITICAL (5) | Never dropped — blocking send |

**Circuit breaker sampling (D-19):** When `breaker.IsOpen()`, only 10% of traffic reaches queue (`rand() <= 0.10` passes through).

**Key interfaces:**
```go
type Evaluator interface {
    Evaluate(ctx context.Context, sig *v1.ArgusSignal) ([]engine.MatchResult, error)
}
type AlertWriter interface {
    WriteAlert(ctx context.Context, m engine.MatchResult) error
}
```

**Severity mapping (corrected from plan):**
- Proto enum: `SEVERITY_UNSPECIFIED=0, INFO=1, LOW=2, MEDIUM=3, HIGH=4, CRITICAL=5`
- SeverityInt maps to those integers directly
- SeverityLabel returns `"info"/"low"/"medium"/"high"/"critical"/"unspecified"`

**Tests:** 6 — DropLow, NeverDropCritical, SamplingWhenBreakerOpen, EvaluatesAndWritesAlert, SeverityInt_AllEnums, SeverityLabel

**Commit:** `60cc393`

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Severity enum values differ from plan assumption**
- **Found during:** Task 3 implementation
- **Issue:** Plan assumed `Severity_SEVERITY_LOW=1` and `Severity_SEVERITY_CRITICAL=5`. Actual proto enum: `Severity_INFO=1, Severity_LOW=2, ..., Severity_CRITICAL=5`. No `Severity_SEVERITY_LOW` constant exists.
- **Fix:** SeverityInt uses actual enum constants. Drop policy threshold adjusted: `sevInt <= 2` covers INFO+LOW, `sevInt == 3` is MEDIUM.
- **Files modified:** `internal/detection/worker/drop_policy.go`, `drop_policy_test.go`

**2. [Rule 1 - Bug] AlertWriter signature has no signal parameter**
- **Found during:** Task 3 implementation
- **Issue:** Plan showed `WriteAlert(ctx, match, sig)` but actual `pipeline.AlertWriter` interface and `ingest.AlertRouter` both use `WriteAlert(ctx, engine.MatchResult)` — signal is embedded in MatchResult.
- **Fix:** `AsyncDetectionWorker.AlertWriter` interface uses single `MatchResult` parameter.
- **Files modified:** `internal/detection/worker/async_worker.go`

**3. [Rule 2 - Enhancement] Added Evaluator interface**
- **Found during:** Task 3 design
- **Issue:** Plan referenced `*engine.DetectionEngine` but test stubs need an interface.
- **Fix:** Defined `Evaluator` interface in worker package mirroring `Evaluate(ctx, sig)` — allows test stubs without circular imports.

---

## Known Stubs

None — all packages are fully implemented with no placeholder data or TODO fields.

---

## Verification

```
go build ./...                                           EXIT 0
go test ./internal/detection/breaker/...                PASS (6 tests)
go test ./internal/detection/worker/...                 PASS (6 tests)
go test ./internal/metrics/ -run TestDetection          PASS (2 tests)
```

Note: `TestMetricsNaming` in `internal/metrics` was pre-existing failure before this plan (verified via git stash). Out of scope.

## Self-Check: PASSED

Files verified:
- `internal/detection/breaker/breaker.go` — FOUND
- `internal/detection/breaker/breaker_test.go` — FOUND
- `internal/detection/worker/async_worker.go` — FOUND
- `internal/detection/worker/async_worker_test.go` — FOUND
- `internal/detection/worker/drop_policy.go` — FOUND
- `internal/detection/worker/drop_policy_test.go` — FOUND
- `internal/metrics/detection.go` — FOUND
- `internal/metrics/detection_test.go` — FOUND

Commits verified: `319d04f`, `476c9b6`, `60cc393` — all present in git log.
