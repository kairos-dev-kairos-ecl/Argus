# Signal Taxonomy — Argus XDR Layer Model

Argus XDR organises every observation from an LLM-integrated system into one of ten layers (L1–L10), plus a decision layer (LDecision). The layer tells you *where* in the stack the event originated. The category tells you *what kind* of event it was.

This document is the canonical reference for the layer model. All protobuf field definitions derive from this taxonomy.

---

## The Eleven Layers

```
┌─────────────────────────────────────────────────────────────┐
│  LDecision  Final judgement / policy enforcement            │
├─────────────────────────────────────────────────────────────┤
│  L10  Application       End-user application behaviour      │
│  L9   Orchestration     Agent / multi-model coordination    │
│  L8   Data Access       Retrieval, RAG, external stores     │
│  L7   Tool Use          Function calls, plugin execution    │
│  L6   Integration       External API calls                  │
│  L5   Output Decoding   Sampling, formatting, streaming     │
│  L4   Inference         Forward pass, compute               │
│  L3   Model Loading     Weights, quantisation, adapters     │
│  L2   Runtime           ML framework internals              │
│  L1   Hardware          GPU/CPU/memory at the system level  │
└─────────────────────────────────────────────────────────────┘
```

### L1 — Hardware

The physical compute substrate. Signals here describe GPU/CPU utilisation, thermal state, memory bandwidth, and NUMA topology. Hardware signals are the only source of ground truth on *what the machine was doing* during inference.

**Key fields:** `gpu_id`, `gpu_utilization_pct`, `vram_used_bytes`, `vram_total_bytes`, `cpu_utilization_pct`, `memory_used_bytes`, `thermal_zone`, `power_draw_watts`, `pcie_bandwidth_gbps`

**Typical events:** GPU OOM, thermal throttle, NUMA migration, PCIe bandwidth saturation

---

### L2 — Runtime

The ML framework layer (PyTorch, JAX, TensorRT). Signals here describe framework-level operations, CUDA kernel launches, memory allocation patterns, and gradient activity.

**Key fields:** `framework`, `framework_version`, `cuda_version`, `operation_name`, `kernel_name`, `memory_allocated_bytes`, `memory_reserved_bytes`, `device_type`, `mixed_precision_mode`

**Typical events:** CUDA OOM, autocast overflow, checkpoint save/restore, framework version mismatch

---

### L3 — Model Loading

The lifecycle of model weights: loading, unloading, quantisation, adapter application. Signals here capture what model was loaded, how, and whether it matched expectations.

**Key fields:** `model_id`, `model_version`, `quantization`, `adapter_ids[]`, `load_duration_ms`, `model_size_bytes`, `checksum`, `source_uri`, `cache_hit`

**Typical events:** Model load (cache hit/miss), quantisation applied, adapter merge, checksum mismatch, unexpected model version

---

### L4 — Inference

The forward pass itself. Each signal captures one inference request — its inputs, outputs, latencies, token counts, and compute costs.

**Key fields:** `model_id`, `request_id`, `prompt_tokens`, `completion_tokens`, `total_tokens`, `latency_ms`, `time_to_first_token_ms`, `tokens_per_second`, `temperature`, `top_p`, `finish_reason`, `batch_size`, `cache_type`, `kv_cache_hit_rate`

**Typical events:** Slow inference, token budget exceeded, unexpected finish reason, KV cache miss spike

---

### L5 — Output Decoding

Post-inference processing: sampling strategy, streaming, format conversion, output filtering. Signals here let you audit what was done *to* the raw model output before it reached the caller.

**Key fields:** `decoding_strategy`, `beam_width`, `repetition_penalty`, `length_penalty`, `output_format`, `stream_chunk_index`, `filter_applied`, `filter_reason`, `output_tokens`, `truncated`

**Typical events:** Repetition penalty applied, output truncated, streaming interrupted, format conversion failed

---

### L6 — Integration

Calls to external APIs and services initiated by the inference system (not by user-facing application code). This is where third-party dependencies become visible in the signal stream.

**Key fields:** `provider`, `endpoint`, `method`, `status_code`, `latency_ms`, `request_bytes`, `response_bytes`, `retry_count`, `error_code`, `auth_method`, `rate_limited`

**Typical events:** API timeout, rate limit hit, auth failure, unexpected response schema

---

### L7 — Tool Use

Function calls, plugin invocations, and code execution initiated by the model. This layer makes tool-using agents auditable — every tool call is a signal.

**Key fields:** `tool_name`, `tool_version`, `tool_call_id`, `input_summary`, `output_summary`, `execution_duration_ms`, `success`, `error_message`, `sandboxed`, `permission_level`, `parent_tool_call_id`

**Typical events:** Tool call succeeded/failed, unexpected tool invoked, permission escalation attempt, sandboxing violation

---

### L8 — Data Access

Retrieval operations — vector search, database queries, file reads, RAG pipelines. Every time the model or agent reaches into an external data store, L8 captures it.

**Key fields:** `store_type`, `store_id`, `query_summary`, `result_count`, `top_similarity_score`, `query_latency_ms`, `bytes_retrieved`, `filtered_results`, `access_control_applied`, `data_classification`

**Typical events:** Vector search (high/low recall), access control applied, sensitive data retrieved, query latency spike

---

### L9 — Orchestration

Multi-model and multi-agent coordination. Signals here describe how one model or agent delegates to another — the backbone of complex agentic workflows.

**Key fields:** `orchestrator_id`, `agent_id`, `agent_role`, `step_index`, `total_steps`, `delegation_target`, `delegation_method`, `context_window_tokens`, `memory_strategy`, `plan_id`, `loop_detected`

**Typical events:** Agent handoff, loop detected, context window approaching limit, plan step failed, orchestration timeout

---

### L10 — Application

The end-user application layer — session management, user intent, conversation state, and business-logic events that frame why the LLM is being called.

**Key fields:** `session_id`, `conversation_id`, `user_id`, `user_intent`, `application_id`, `interface_type`, `locale`, `feature_flag`, `ab_test_group`, `response_time_ms`, `user_feedback_score`

**Typical events:** Session start/end, user feedback collected, feature flag evaluated, A/B group assigned, slow response UX threshold crossed

---

### LDecision

The policy enforcement layer. Signals here record final decisions made by the detection and response engine — which rules fired, what risk score was produced, and what action was taken.

**Key fields:** `decision_id`, `rule_id`, `rule_name`, `risk_score`, `action_taken`, `action_reason`, `suppressed`, `analyst_override`, `confidence`, `evidence_signals[]`

**Typical events:** Alert raised, signal suppressed, analyst override applied, risk score threshold crossed

---

## The ArgusSignal Envelope

Every signal, regardless of layer, is wrapped in the same envelope:

```protobuf
message ArgusSignal {
  string signal_id       = 1;   // ULID — time-sortable unique ID
  string trace_id        = 2;   // Groups signals from one request flow
  string conversation_id = 3;   // Groups signals from one user session
  int32  layer           = 4;   // 1–10 or 99 (LDecision)
  string category        = 5;   // Dotted namespace: "inference.latency"
  Severity severity      = 6;   // DEBUG / INFO / WARN / ERROR / CRITICAL
  google.protobuf.Timestamp timestamp = 7;

  SourceContext source   = 8;   // app_id, host, process, environment
  oneof layer_context { ... }   // L1Context, L2Context, ... L10Context
  LDecisionContext decision_context = 20;
}
```

The `layer_context` oneof ensures that fields are type-safe and layer-specific. You cannot accidentally put an L4 inference field on an L1 hardware signal.

---

## Signal Categories

Categories use a dotted namespace convention: `<layer-domain>.<event-type>`. Examples:

| Layer | Category | Meaning |
|-------|----------|---------|
| L1 | `hardware.gpu.oom` | GPU ran out of memory |
| L1 | `hardware.thermal.throttle` | Thermal throttling started |
| L3 | `model.load.cache_miss` | Model loaded from disk (not cache) |
| L4 | `inference.latency.p99_exceeded` | Inference exceeded p99 threshold |
| L4 | `inference.tokens.budget_exceeded` | Token count over limit |
| L5 | `output.filter.applied` | Content filter triggered |
| L7 | `tool.call.permission_escalation` | Tool attempted to exceed granted scope |
| L8 | `data.retrieval.sensitive_classification` | Sensitive data returned from store |
| L9 | `orchestration.loop.detected` | Agent loop detected |
| L10 | `application.session.start` | New user session |
| LDecision | `decision.alert.raised` | Detection rule fired |

---

## Severity Levels

| Level | Value | Use |
|-------|-------|-----|
| `DEBUG` | 0 | High-volume diagnostic data — usually filtered at ingest |
| `INFO` | 1 | Normal operational events |
| `WARN` | 2 | Anomalous but not immediately actionable |
| `ERROR` | 3 | Failure events — should produce a signal at minimum |
| `CRITICAL` | 4 | Requires immediate operator attention |

Severity feeds into the detection engine's baseline scorer. Signals with severity ≥ WARN are always persisted; DEBUG signals may be sampled or dropped under load.

---

## Trace Correlation

`trace_id` is the primary correlation key. All signals from a single request flow — from the application layer down to hardware — share the same `trace_id`. This enables the behavioural trace view in the TUI and dashboard:

```
trace_id: 6154dd0c-a9e1-471e-8b1b-7d789b9dbf0d
  └─ L10  application.session.start         t=0ms
  └─ L9   orchestration.agent.delegated     t=2ms
  └─ L4   inference.latency.normal          t=180ms
  └─ L3   model.load.cache_hit              t=181ms
  └─ L1   hardware.gpu.utilization.normal   t=182ms
  └─ L5   output.filter.not_applied         t=340ms
```

`conversation_id` groups traces across multiple turns of the same user session. The session baseline engine uses conversation-scoped data to compute drift scores.

---

## Using the SDK

```python
from sdk.signal_builder import SignalBuilder

signal = (
    SignalBuilder()
    .layer(4)                              # L4 — Inference
    .category("inference.latency.p99_exceeded")
    .severity("WARN")
    .trace_id("6154dd0c-...")
    .source(app_id="my-llm-app", host="worker-01")
    .l4_context(
        model_id="qwen2.5:1.5b",
        latency_ms=4800,
        prompt_tokens=512,
        completion_tokens=128,
        finish_reason="stop",
    )
    .build()
)
```

See `docs/getting-started.md` for a complete integration walkthrough.
