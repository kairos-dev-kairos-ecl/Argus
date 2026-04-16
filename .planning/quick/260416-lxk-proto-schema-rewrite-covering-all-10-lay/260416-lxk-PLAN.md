---
phase: quick
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - proto/argus/v1/signal.proto
  - internal/storage/schema.go
  - sdk/signal_builder.py
autonomous: true
requirements: [REQ-P1-01, REQ-P1-02, REQ-P1-03, REQ-P1-04, REQ-P1-05, REQ-P1-06, REQ-P1-07, REQ-P1-08, REQ-P1-09, REQ-P1-10, REQ-P1-11, REQ-P1-12, REQ-P1-13, REQ-P1-14]
must_haves:
  truths:
    - "All 10 ContextL* messages plus ContextLDecision have complete field definitions (no placeholders)"
    - "Go stubs regenerate cleanly from the new proto"
    - "ClickHouse DDL columns match proto fields for all layers"
    - "Python SDK can build signals for all 11 context types"
  artifacts:
    - path: "proto/argus/v1/signal.proto"
      provides: "Complete 10-layer + LDecision proto schema"
      contains: "message ContextL1"
    - path: "internal/storage/schema.go"
      provides: "ClickHouse DDL aligned to new proto"
      contains: "ctx_l1_cpu_usage_pct"
    - path: "sdk/signal_builder.py"
      provides: "Builder methods for all context types"
      contains: "def _set_context"
  key_links:
    - from: "proto/argus/v1/signal.proto"
      to: "gen/go/argus/v1/"
      via: "buf generate"
      pattern: "buf generate"
    - from: "internal/storage/schema.go"
      to: "proto/argus/v1/signal.proto"
      via: "column names match proto field names"
      pattern: "ctx_l[0-9]"
    - from: "sdk/signal_builder.py"
      to: "proto/argus/v1/signal.proto"
      via: "signal_pb2 imports"
      pattern: "signal_pb2\\.ContextL"
---

<objective>
Rewrite proto/argus/v1/signal.proto to replace all 7 placeholder ContextL* messages with complete field definitions, extend L5/L7/L8 with missing fields, add audit chain fields to ContextLDecision, then cascade changes to ClickHouse DDL and Python SDK.

Purpose: The proto schema is the contract for the entire platform. Placeholder stubs block build stabilization (Phase 2) and prevent meaningful signal ingestion.
Output: Complete proto, regenerated Go stubs, aligned ClickHouse DDL, updated SDK builder.
</objective>

<execution_context>
@C:/Users/Drupad/.claude/get-shit-done/workflows/execute-plan.md
@C:/Users/Drupad/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@proto/argus/v1/signal.proto
@internal/storage/schema.go
@sdk/signal_builder.py
</context>

<tasks>

<task type="auto">
  <name>Task 1: Rewrite signal.proto with complete layer definitions</name>
  <files>proto/argus/v1/signal.proto</files>
  <action>
Rewrite proto/argus/v1/signal.proto. Keep all existing envelope messages (ArgusSignal, Source, Provider, Enrichment, ThreatIntelMatch, GeoData, LogProbEntry, Alternative, LatencyBreakdown, BaselineProfile) and enums (Layer, Severity, DataClassification) UNCHANGED.

Replace/extend the following messages with the EXACT field definitions from the locked design decisions below. Do NOT change field numbers on existing non-placeholder fields in L5, L7, L8.

**REPLACE placeholders (REQ-P1-01 through REQ-P1-04, REQ-P1-06, REQ-P1-09, REQ-P1-10):**
- ContextL1: 24 fields (cpu_usage_pct through process_memory_mb) per design decision
- ContextL2: 36 fields (model_id through device_map) per design decision. Note: field numbers skip 15-19 and 23 intentionally
- ContextL3: 27 fields (input_tokens through has_special_tokens) per design decision. Note: field numbers skip 2-3 and 16-18 intentionally
- ContextL4: 25 fields (kv_cache_used_mb through compute_dtype) per design decision. Note: field numbers skip 9-10 intentionally
- ContextL6: Replace placeholder entirely. Add new message SafetyCategoryScore before ContextL6. ContextL6 has 29 fields per design decision. Note: field numbers skip 5-10 and 14 intentionally
- ContextL9: 29 fields (method through endpoint_alias) per design decision
- ContextL10: 24 fields (user_id through deployment_id) per design decision

**EXTEND existing messages (REQ-P1-05, REQ-P1-07, REQ-P1-08):**
- ContextL5: Keep existing fields (operation through tps, field nums 1-14). Add new fields starting at field number 15: top_k (int32), top_p (float, BUT field 7 is already top_p -- rename new one to nucleus_p or SKIP since top_p already exists), min_p (float)=16, repetition_penalty (float)=17, presence_penalty (float)=18, frequency_penalty (float)=19, seed (int64)=20, stop_sequences (repeated string)=21, output_repetition_score (float)=22, distinct_ngrams_ratio (float)=23, truncated (bool)=24, finish_reason_detail (string)=25.
  IMPORTANT: ContextL5 already has top_p at field 7. Do NOT add a duplicate. The new field list for L5 is: top_k=15, min_p=16, repetition_penalty=17, presence_penalty=18, frequency_penalty=19, seed=20, stop_sequences=21, output_repetition_score=22, distinct_ngrams_ratio=23, truncated=24, finish_reason_detail=25.
- ContextL7: Keep existing fields (operation through latency_breakdown, field nums 1-12). Add new fields starting at 13: query_cache_hit=13, query_cache_key=14, citations_used=15, citation_overlap_score=16, index_name=17, index_type=18, index_document_count=19, index_freshness_hours=20, reranker_score=21, reranker_model=22, hybrid_search=23, sparse_weight=24, dense_weight=25.
- ContextL8: Keep existing fields (operation through data_flow_tags, field nums 1-12). Add new fields starting at 13: task_id=13, parent_task_id=14, task_depth=15, subtask_count=16, memory_operation=17, memory_items_read=18, memory_items_written=19, code_language=20, code_executed=21, code_exit_code=22, code_execution_time_ms=23, sandboxed=24, capabilities_used (repeated string)=25, orchestrator_type=26.

**EXTEND ContextLDecision (REQ-P1-11):**
- Keep existing fields (decision through evaluation_time_ms, field nums 1-7). Add: policy_id=8, triggering_rule_id=9, triggering_signal_id=10, failed_open=11, decision_trace_id=12, alert_threshold=13, matched_indicators (repeated string)=14.

All new non-required fields MUST be `optional` (except repeated fields which are inherently optional in proto3). Use the exact types from the design decisions.
  </action>
  <verify>
    <automated>cd C:/Users/Drupad/ArgusXDR/proto && buf lint 2>&1; echo "EXIT:$?"</automated>
  </verify>
  <done>All 11 context messages have complete field definitions. No placeholder fields remain. buf lint passes.</done>
</task>

<task type="auto">
  <name>Task 2: Regenerate Go stubs, align ClickHouse DDL, update Python SDK</name>
  <files>internal/storage/schema.go, sdk/signal_builder.py</files>
  <action>
**Step 1 — Regenerate Go stubs (REQ-P1-12):**
Run `cd C:/Users/Drupad/ArgusXDR && make proto-generate`. If buf is not installed, install via `go install github.com/bufbuild/buf/cmd/buf@latest` or download binary. Verify gen/go/argus/v1/signal.pb.go is regenerated and contains all new message types.

**Step 2 — Align ClickHouse DDL (REQ-P1-13):**
Rewrite the layer context columns section of SignalsTableDDL in internal/storage/schema.go. Replace the current sparse columns with comprehensive columns matching the new proto. Use this naming convention: `ctx_l{N}_{snake_case_field_name}`.

Column type mapping from proto types:
- string -> Nullable(String)
- int32 -> Nullable(Int32)
- int64 -> Nullable(Int64)
- float -> Nullable(Float32)
- double -> Nullable(Float64)
- bool -> Nullable(UInt8)
- repeated string -> Array(String)
- repeated float -> Array(Float32)
- map<string,string> -> Map(String, String)
- SafetyCategoryScore (repeated message) -> Nullable(String) (store as JSON)

Include ALL fields from ALL layers. For L5, include existing fields (operation, output_tokens, input_tokens, total_tokens, finish_reason, temperature, top_p, mean_logprob, min_logprob, entropy_mean, entropy_variance, ttft_ms, tps) PLUS the new extension fields. Same approach for L7 and L8.

Add LDecision columns including the new audit chain fields.

Keep all non-layer columns (identity, source, classification, temporal, relationships, provider, enrichment, governance, version) UNCHANGED. Keep the ENGINE, ORDER BY, PARTITION BY, TTL, SETTINGS lines UNCHANGED.

**Step 3 — Update Python SDK (REQ-P1-14):**
Rewrite the `_set_context` method in sdk/signal_builder.py to handle ALL 11 context types (L1-L10 + LDecision). For each layer:
- Create the appropriate signal_pb2.ContextL{N} message
- Map dict keys to proto fields using `.get()` with sensible defaults
- For optional fields, only set if present in context dict (use conditional assignment)
- For repeated fields, use list defaults
- For SafetyCategoryScore in L6, accept a list of dicts and convert to SafetyCategoryScore messages
- For map fields (L8 tool_arguments, L10 feature_flags, custom_attributes), use `.update()` pattern

Keep the existing `build()` method signature unchanged. Keep `_ulid()` unchanged.
  </action>
  <verify>
    <automated>cd C:/Users/Drupad/ArgusXDR && python -c "from sdk.signal_builder import SignalBuilder; print('SDK imports OK')" 2>&1; echo "EXIT:$?"</automated>
  </verify>
  <done>Go stubs regenerated in gen/go/. ClickHouse DDL has columns for all proto fields across all 11 context types. Python SDK _set_context handles all 11 layers with proper field mapping.</done>
</task>

</tasks>

<verification>
1. `cd proto && buf lint` passes with no errors
2. `gen/go/argus/v1/signal.pb.go` exists and contains ContextL1 through ContextL10 + ContextLDecision + SafetyCategoryScore
3. `grep -c "placeholder" proto/argus/v1/signal.proto` returns 0
4. `grep -c "ctx_l1_" internal/storage/schema.go` returns 20+ (L1 has 24 fields, not all need columns but most do)
5. Python SDK imports without error
</verification>

<success_criteria>
- Zero placeholder messages remain in signal.proto
- All 14 requirements (REQ-P1-01 through REQ-P1-14) addressed
- Go stubs compile from new proto
- ClickHouse DDL covers all layer context fields
- Python SDK builds signals for all 11 context types
</success_criteria>

<output>
After completion, create `.planning/quick/260416-lxk-proto-schema-rewrite-covering-all-10-lay/260416-lxk-SUMMARY.md`
</output>
