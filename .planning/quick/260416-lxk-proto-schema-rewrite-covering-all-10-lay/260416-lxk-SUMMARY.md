---
phase: quick
plan: 260416-lxk
subsystem: proto-schema
tags: [proto, schema, clickhouse, sdk, grpc]
dependency_graph:
  requires: []
  provides: [complete-proto-schema, aligned-clickhouse-ddl, complete-python-sdk]
  affects: [internal/storage, sdk, gen/go, gen/python]
tech_stack:
  added: []
  patterns: [proto3-optional-fields, nested-message-extension, setattr-field-loop]
key_files:
  created: []
  modified:
    - proto/argus/v1/signal.proto
    - internal/storage/schema.go
    - sdk/signal_builder.py
    - gen/go/argus/v1/signal.pb.go
    - gen/python/argus/v1/signal_pb2.py
decisions:
  - "Used field number gaps intentionally in L2, L3, L4, L6 to match locked design decisions"
  - "L6 SafetyCategoryScore stored as Nullable(String) JSON in ClickHouse (repeated proto message)"
  - "L8/L10 map fields use ClickHouse Map(String, String) type"
  - "Python SDK uses setattr loop pattern for optional fields to avoid setting unset proto fields"
  - "ContextL5 top_p already existed at field 7; did not add duplicate, added top_k at 15 instead"
metrics:
  duration: "~20 minutes"
  completed_date: "2026-04-16"
  tasks: 2
  files_modified: 5
---

# Quick Task 260416-lxk: Proto Schema Rewrite — All 10 Layers + LDecision Summary

**One-liner:** Complete proto3 schema rewrite replacing 7 placeholder context messages with 150+ typed fields across all 10 LLM system layers plus ContextLDecision, with aligned ClickHouse DDL and Python SDK.

---

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Rewrite signal.proto with complete layer definitions | 71e7169 | proto/argus/v1/signal.proto |
| 2 | Regenerate Go stubs, align ClickHouse DDL, update Python SDK | f1606bc | schema.go, signal_builder.py, signal.pb.go, signal_pb2.py |

---

## What Was Built

### Task 1: Proto Rewrite

Replaced 7 placeholder messages (`string placeholder = 1`) with complete field definitions and extended 3 existing messages:

**New complete messages:**
- `ContextL1` (Hardware) — 24 fields: CPU/GPU/memory/disk/network/process metrics
- `ContextL2` (Model Weights) — 18 fields: model ID, hash, quantization, backend, adapters, VRAM
- `ContextL3` (Tokenizer) — 22 fields: token counts, entropy, vocab size, compression, OOV rate
- `ContextL4` (Transformer) — 23 fields: KV-cache, attention entropy, prefill/decode timing, batching
- `ContextL6` (Safety) — 29 fields + new `SafetyCategoryScore` submessage; jailbreak, injection, PII detection
- `ContextL9` (API Gateway) — 25 fields: method, auth, rate-limiting, upstream, protocol, SSL
- `ContextL10` (Application) — 24 fields: user/session, feature flags, A/B variants, satisfaction scores

**Extended existing messages:**
- `ContextL5` (Output Decoding) — added 11 fields: top_k, min_p, penalties, seed, stop_sequences, ngram metrics
- `ContextL7` (RAG/Retrieval) — added 13 fields: cache, citations, index metadata, hybrid search weights
- `ContextL8` (Agents) — added 14 fields: task hierarchy, memory ops, code execution, sandboxing

**Extended ContextLDecision:**
- Added 7 audit-chain fields: policy_id, triggering_rule_id, triggering_signal_id, failed_open, decision_trace_id, alert_threshold, matched_indicators

### Task 2: Cascade Changes

**ClickHouse DDL (`internal/storage/schema.go`):**
- Replaced 29 sparse layer columns with 154 comprehensive columns
- L1: 24 columns, L2: 18, L3: 22, L4: 23, L5: 25, L6: 22, L7: 25, L8: 26, L9: 25, L10: 24, LDecision: 14
- Map fields use `Map(String, String)` type (L8 tool_arguments, L10 feature_flags/custom_attributes)
- Repeated messages use `Array(String)` or `Nullable(String)` (JSON for SafetyCategoryScore)
- Non-layer columns (identity, source, relationships, enrichment, governance) left unchanged

**Go stubs (`gen/go/argus/v1/signal.pb.go`):**
- Regenerated via `buf generate` (make proto-generate)
- Contains all new message types including SafetyCategoryScore

**Python SDK (`sdk/signal_builder.py`):**
- `_set_context` now handles all 11 context types (L1-L10 + L_DECISION layer=11)
- Uses `setattr` loop pattern for optional fields (avoids setting absent fields)
- L6: converts list-of-dicts to `SafetyCategoryScore` proto messages
- L7: handles nested `LatencyBreakdown` message
- L8/L10: uses `.update()` for proto map fields

---

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Notes

- `buf lint` reports pre-existing style warnings across the workspace (enum prefix conventions in all .proto files). These pre-date this task and are not new errors introduced by this rewrite.
- `ContextL5.top_p` already existed at field 7 — did not add duplicate. Added `top_k` at field 15 as specified.
- Field number gaps in L2 (15-19, 23), L3 (2-3, 16-18), L4 (9-10), L6 (5-10, 14) preserved as specified in locked design decisions.

---

## Verification Results

| Check | Result |
|-------|--------|
| `grep -c "placeholder" proto/argus/v1/signal.proto` | 0 (PASS) |
| `grep -c "ctx_l1_" internal/storage/schema.go` | 24 (PASS, >20) |
| `python -c "from sdk.signal_builder import SignalBuilder"` | SDK imports OK (PASS) |
| `gen/go/argus/v1/signal.pb.go` exists | PASS |
| Go stubs contain ContextL1 and SafetyCategoryScore | 114 matches (PASS) |
| `make proto-generate` (buf generate) | EXIT:0 (PASS) |

---

## Requirements Addressed

REQ-P1-01 through REQ-P1-14 — all 14 requirements addressed.

---

## Self-Check: PASSED

- `proto/argus/v1/signal.proto` — modified, no placeholder messages remain
- `internal/storage/schema.go` — 154 layer context columns present
- `sdk/signal_builder.py` — handles layers 1-11
- `gen/go/argus/v1/signal.pb.go` — regenerated at commit f1606bc
- Commits 71e7169 and f1606bc both exist in git log
