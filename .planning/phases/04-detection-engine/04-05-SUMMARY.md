---
phase: 04-detection-engine
plan: 05
subsystem: ingest/alert_router
tags: [fingerprinting, dedup, suppression, jsonb, d04, d05, d06, d07]
dependency_graph:
  requires: [04-01]
  provides: [D-04, D-05, D-06, D-07 compliance in AlertRouter]
  affects: [internal/ingest/alert_router.go, internal/ingest/alert_router_test.go]
tech_stack:
  patterns: [ON CONFLICT upsert, sha256 fingerprinting, JSONB context, auto-suppress lifecycle]
key_files:
  modified:
    - internal/ingest/alert_router.go
    - internal/ingest/alert_router_test.go
decisions:
  - entity field uses Source.AppId + ":" + SessionId (GetSessionId()) — most specific per-caller identifier available in proto
  - normalized_payload = fmt.Sprintf("%d|%s|%d", Layer, Category, Severity) — deterministic across runs
  - suppressThreshold default 100; configurable via struct field set after NewAlertRouter
  - Integration tests (TestUpsertAlert_*) skip under -short; real DB tests deferred to Wave 4
metrics:
  duration: 15m
  completed_date: "2026-04-19"
  tasks: 2
  files: 2
---

# Phase 04 Plan 05: AlertRouter Fingerprinting + Lifecycle Summary

AlertRouter brought to D-04/D-05/D-06/D-07 compliance: sha256(rule_id|entity|normalized_payload) fingerprint, mandatory trace_id enforcement, JSONB context persistence, and SUPPRESSED auto-trigger at configurable threshold.

## What Was Built

### Task 1: Fingerprint Computation + Trace Linkage (D-05, D-06)

`computeRouterFingerprint` rewritten to:

```go
entity  = Source.AppId + ":" + GetSessionId()
payload = fmt.Sprintf("%d|%s|%d", sig.Layer, sig.Category, sig.Severity)
hash    = sha256(ruleID + "|" + entity + "|" + payload)
```

`WriteAlert` enforces D-06 at the top of every call — empty `trace_id` returns `ErrMissingTraceID` with no DB write attempted.

### Task 2: JSONB Context + SUPPRESSED Auto-Trigger (D-04, D-07)

- `upsertAlert` SQL: `ON CONFLICT (fingerprint) DO UPDATE SET signal_count = alerts.signal_count + 1, last_seen_at = NOW(), signal_ids = array_append(...)`
- After upsert, if `signal_count > suppressThreshold` (default 100): `UPDATE alerts SET status='suppressed' WHERE id=$1 AND status='open'`
- `context` JSONB = `{ "rule_action": {...}, "tier": N, "reason": "..." }`
- `kairos_decision` JSONB = marshalled PolicyDecision or null

### Key Constants

| Name | Value | Purpose |
|------|-------|---------|
| `defaultSuppressThreshold` | 100 | Signals at which alert transitions to SUPPRESSED |
| `dedupWindow` | 15 minutes | Redis dedup key TTL |

## Tests

| Test | Status |
|------|--------|
| TestFingerprint_DifferentLayers_DifferentHashes | PASS |
| TestFingerprint_SameInputs_SameHash | PASS |
| TestFingerprint_IncludesRuleID | PASS |
| TestFingerprintComposition | PASS |
| TestWriteAlert_MissingTraceID_Rejects | PASS |
| TestSuppressThreshold_DefaultIs100 | PASS |
| TestUpsertAlert_IncrementsCount | SKIP (-short, requires PostgreSQL) |
| TestUpsertAlert_AutoSuppressAbove100 | SKIP (-short, requires PostgreSQL) |
| TestUpsertAlert_PersistsContextJSONB | SKIP (-short, requires PostgreSQL) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test file used non-existent enum names**
- **Found during:** Task 1 verification
- **Issue:** Test file referenced `v1.Severity_SEV_LOW`, `v1.Severity_SEV_HIGH`, `v1.Severity_SEV_CRITICAL`, `v1.Severity_SEV_MEDIUM`, `v1.Layer_L3_RUNTIME`, `v1.Layer_L5_INFERENCE` — none of which exist in the generated proto. Actual names are `Severity_LOW`, `Severity_HIGH` etc. and `Layer_L3_TOKENIZER`, `Layer_L5_OUTPUT_DECODING`.
- **Fix:** Updated all 7 incorrect enum references to match actual generated values.
- **Files modified:** `internal/ingest/alert_router_test.go`
- **Commit:** 20d7c3d

## Known Stubs

None — all functional behavior is wired. PostgreSQL-dependent tests are intentionally skipped under `-short` and documented for Wave 4 integration.

## Self-Check: PASSED

- `internal/ingest/alert_router.go` — exists, contains `sha256.Sum256`, `ErrMissingTraceID`, `ON CONFLICT (fingerprint)`, `status='suppressed'`, `suppressThreshold`
- `internal/ingest/alert_router_test.go` — exists, contains all required test functions
- Commit `20d7c3d` — confirmed in git log
- `go build ./...` — exits 0
- `go test ./internal/ingest/... -short` — all pass (integration tests skipped as designed)
