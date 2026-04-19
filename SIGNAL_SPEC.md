# Argus XDR - 10-Layer LLM Signal Specification

## End-to-End Signal Flow with Qwen 3.5 0.8B

### L1: HARDWARE (Infrastructure metrics)
**Triggered**: System startup, every 30s during inference
**Signals Emitted**:
```protobuf
context_l1: {
  cpu_percent: float      // CPU utilization %
  memory_used_mb: int64   // RAM used in MB
  gpu_utilization_pct: float  // GPU % if available
}
```
**Data Source**: `psutil` (CPU, memory) + `torch.cuda` (GPU)
**Expected Frequency**: 1 signal per 30s

---

### L2: MODEL_WEIGHTS (Model metadata & state)
**Triggered**: Model load, weight changes, quantization checks
**Signals Emitted**:
```protobuf
context_l2: {
  model_id: string        // "qwen-3.5-0.8b"
  model_hash: string      // SHA256 of weights
  quantization: string    // "int4", "int8", "fp32"
}
```
**Data Source**: Model checkpoint metadata, quantization config
**Expected Frequency**: 1 signal at startup, 1 on change

---

### L3: TOKENIZER (Tokenization process)
**Triggered**: Each prompt encoding
**Signals Emitted**:
```protobuf
context_l3: {
  input_token_count: int64    // Tokens from prompt
  output_token_count: int64   // Max tokens to generate
  truncated: bool             // Was input truncated?
}
```
**Data Source**: Tokenizer output length
**Expected Frequency**: 1 signal per inference request

---

### L4: TRANSFORMER (Attention & computation)
**Triggered**: During forward pass (sample every N layers)
**Signals Emitted**:
```protobuf
context_l4: {
  attention_entropy: float     // Shannon entropy of attention weights
  kv_cache_hit_rate: float     // Reuse % from KV cache
}
```
**Data Source**: Attention weights, KV cache stats
**Expected Frequency**: 1 signal per 10 layers (sampling)

---

### L5: OUTPUT_DECODING (Logits & sampling)
**Triggered**: Token generation loop
**Signals Emitted**:
```protobuf
context_l5: {
  mean_logprob: float         // Average log probability
  top_logprob: float          // Highest logit
  finish_reason: string       // "max_tokens", "stop", "length"
  temperature: float          // Sampling temperature
  top_p: float                // Nucleus sampling threshold
  ttft_ms: float              // Time to first token
  tps: float                  // Tokens per second
}
```
**Data Source**: Generation loop metrics
**Expected Frequency**: 1 signal at generation end

---

### L6: SAFETY (Safety filters & guardrails)
**Triggered**: After each generation chunk
**Signals Emitted**:
```protobuf
context_l6: {
  placeholder: string  // Reserved for safety checks
}
```
**Data Source**: Content filter results (if implemented)
**Expected Frequency**: 1 signal per generation

---

### L7: RAG_RETRIEVAL (Retrieval-Augmented Generation)
**Triggered**: RAG query phase
**Signals Emitted**:
```protobuf
context_l7: {
  operation: int           // 1=query, 2=retrieve, 3=rank
  query_text: string       // Retrieved context
  results_count: int64     // Number of results
  embedding_model: string  // Model used for embeddings
  vector_index: string     // Index name
  context_window_pct: float // % of context window used
}
```
**Data Source**: RAG system (if enabled)
**Expected Frequency**: 1-3 signals per inference with RAG

---

### L8: AGENTS (Tool use & orchestration)
**Triggered**: Tool calls
**Signals Emitted**:
```protobuf
context_l8: {
  operation: int                   // 1=plan, 2=execute, 3=observe
  tool_name: string                // Tool invoked (e.g., "calculator")
  tool_provider: string            // Provider (e.g., "local", "api")
  tool_result: string              // Tool output
  tool_error: string               // Error if failed
  tool_latency_ms: float           // Execution time
  step_number: int64               // Step in agent loop
  total_steps: int64               // Total steps planned
  data_flow_tags: repeated string  // ["input", "output", "state"]
  permissions_used: repeated string // ["file_read", "network"]
  permissions_requested: repeated string
  tool_arguments: map<string, string>
}
```
**Data Source**: Agent framework hooks
**Expected Frequency**: 1 signal per tool invocation

---

### L9: API_GATEWAY (API boundaries)
**Triggered**: HTTP request/response
**Signals Emitted**:
```protobuf
context_l9: {
  method: string      // "POST", "GET"
  path: string        // "/v1/completions"
  status_code: int64  // HTTP response code
  latency_ms: float   // End-to-end latency
}
```
**Data Source**: FastAPI/HTTP middleware
**Expected Frequency**: 1 signal per API call

---

### L10: APPLICATION (User-level)
**Triggered**: User interactions
**Signals Emitted**:
```protobuf
context_l10: {
  placeholder: string  // Reserved for app-level events
}
```
**Data Source**: Application instrumentation
**Expected Frequency**: 1 signal per user action

---

## Signal Flow Summary

```
User Request
    ↓
[L9] API Gateway receives POST /completions
    ↓
[L1] Hardware metrics captured
[L2] Model loaded/verified
    ↓
[L3] Tokenizer encodes input
    ↓
[L4] Transformer forward pass (sample attention)
    ↓
[L5] Output decoding + sampling
    ↓
[L6] Safety filters applied
    ↓
[L7] (Optional) RAG retrieval if needed
    ↓
[L8] (Optional) Tool invocations
    ↓
[L5] Final token generation complete
    ↓
[L9] API Gateway returns response
    ↓
[L10] Application logs completion
```

---

## Data Validation Checklist

- [ ] All 10 layers emit at least 1 signal during inference
- [ ] Signal timestamps are monotonically increasing within a trace
- [ ] trace_id is consistent across all signals in a request
- [ ] Layer numbers (1-10) are correct in protobuf
- [ ] Signal counts per layer match expectations
- [ ] Enrichment fields are populated (baseline, GeoIP, threat intel)
- [ ] WebSocket delivers signals in real-time
- [ ] Query API returns all signals in ClickHouse
- [ ] Alert detection fires on anomalies (if rules implemented)

---

## Production Readiness Criteria

1. **Correctness**: Every layer emits appropriate signals
2. **Completeness**: All 60+ ClickHouse columns populated
3. **Performance**: <100ms overhead per inference
4. **Reliability**: No dropped signals, proper error handling
5. **Observability**: Full trace reconstruction from signals
