# Argus XDR Data Flow & Architecture

Complete end-to-end data flow documentation for the Argus XDR platform with Qwen 3.5 0.8B integration.

## System Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         QWEN 3.5 0.8B INFERENCE                             │
│                                                                             │
│  ┌────────────────┐      ┌─────────────────┐      ┌──────────────────┐   │
│  │ L1: HARDWARE   │      │  L2: WEIGHTS    │      │ L3: TOKENIZER    │   │
│  │ CPU%, Mem, GPU │      │ ID, Hash, Quant │      │ Token counts     │   │
│  └────────┬────────┘      └────────┬────────┘      └────────┬─────────┘   │
│           │                        │                        │             │
│  ┌────────▼─────────┐      ┌──────▼──────────┐      ┌──────▼────────┐    │
│  │ L4: TRANSFORMER  │      │ L5: DECODING    │      │ L10: APP      │    │
│  │ Attn, KV cache   │      │ Logprobs, TTFT  │      │ User events   │    │
│  └────────┬─────────┘      └────────┬────────┘      └──────┬────────┘    │
│           │                        │                       │              │
│  ┌────────▼──────────────────────────┴───────────────────┬┘              │
│  │ [Optional: L6 SAFETY, L7 RAG, L8 AGENTS]              │              │
│  │                                                        │              │
│  └────────┬─────────────────────────────────────────────┘              │
│           │                                                             │
│  ┌────────▼─────────────────────────────────────────────┐              │
│  │ L9: API_GATEWAY (HTTP request/response metrics)     │              │
│  └────────┬─────────────────────────────────────────────┘              │
│           │                                                             │
└───────────┼─────────────────────────────────────────────────────────────┘
            │ (SDK: emit signals via protobuf)
            ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│              ARGUS SIGNAL PROCESSING PIPELINE (Go Backend)                  │
│                                                                             │
│  HTTP/gRPC Receiver → Ingest Queue → Processor Chain → ClickHouse          │
│                                                                             │
│  ┌──────────────────┐   ┌──────────────────┐   ┌──────────────────────┐   │
│  │ HTTP Receiver    │   │ Signal Buffer    │   │ Processor Pipeline   │   │
│  │ :8080/v1/signals │──→│ (buffered queue) │──→│ 1. Validate schema   │   │
│  │ protobuf payload │   │ ~1000 signals    │   │ 2. Normalize fields  │   │
│  │ (201 Created)    │   │ in-memory        │   │ 3. Correlate traces  │   │
│  └──────────────────┘   └──────────────────┘   │ 4. Enrich (async)    │   │
│                                                │ 5. Score baseline    │   │
│                         ┌────────────────────┤ 6. Tag anomalies     │   │
│                         │                    │ 7. Route to storage  │   │
│                         │                    └──────────┬───────────┘   │
│  ┌──────────────────┐   │                              │               │
│  │ Async Enrichment │   │  ┌──────────────────────────┘               │
│  │ • GeoIP lookup   │───┘  │                                          │
│  │ • Threat intel   │      │ ┌──────────────────────────────────┐    │
│  │ • Baseline Z     │      └→│ ClickHouse (MergeTree engine)    │    │
│  │                  │         │ • 60-column optimized schema    │    │
│  └──────────────────┘         │ • Monthly partitions            │    │
│                               │ • Async batch insert            │    │
│  ┌──────────────────────┐     │ • Real-time WebSocket delivery  │    │
│  │ Signal Broadcaster   │─────→ • Row deduplication (trace_id) │    │
│  │ (WebSocket fan-out)  │     │                                │    │
│  └──────────────────────┘     └────────────────┬───────────────┘    │
│                                                 │                     │
│  ┌──────────────────┐   ┌───────────────────────┴──────────────┐    │
│  │ PostgreSQL       │   │ Redis (ephemeral state)              │    │
│  │ • baseline_      │   │ • trace correlation sets             │    │
│  │   profiles       │   │ • dedup ring buffer                  │    │
│  │ • users, api_    │   │ • GeoIP cache (24h TTL)              │    │
│  │   keys           │   │ • baseline profiles (5m TTL)         │    │
│  │                  │   │ • alert state (30s TTL)              │    │
│  └──────────────────┘   └──────────────────────────────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────────────┘
            │ (Async aggregation, storage, enrichment)
            ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                    QUERY & DETECTION LAYER (REST API)                       │
│                                                                             │
│  ┌──────────────────────────┐      ┌──────────────────────────┐           │
│  │ Query API (:8080)        │      │ Detection Engine         │           │
│  │ • SQL execution          │      │ • YAML rule evaluation   │           │
│  │ • Signal filtering       │      │ • Incident creation      │           │
│  │ • Trace reconstruction   │      │ • Alert dispatch         │           │
│  └──────────────────────────┘      └──────────────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
            │ (JSON responses)
            ↓
┌─────────────────────────────────────────────────────────────────────────────┐
│                    DASHBOARD (React 18 + TypeScript)                        │
│                                                                             │
│  ┌────────────────────────┐      ┌────────────────────────┐              │
│  │ Signal Stream View     │      │ Trace View             │              │
│  │ • Real-time updates    │      │ • 10-layer hierarchy   │              │
│  │ • WebSocket delivery   │      │ • Span relationships   │              │
│  │ • Layer filtering      │      │ • Context inspection   │              │
│  │ • Severity badges      │      │ • Enrichment display   │              │
│  └────────────────────────┘      └────────────────────────┘              │
│                                                                            │
│  ┌────────────────────────┐      ┌────────────────────────┐              │
│  │ Coverage Map           │      │ Query Console          │              │
│  │ • L1-L10 heatmap       │      │ • SQL editor           │              │
│  │ • Activity timeline    │      │ • ClickHouse execution │              │
│  │ • Metric sparklines    │      │ • Result export        │              │
│  └────────────────────────┘      └────────────────────────┘              │
│                                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Layer-by-Layer Data Flow

### Layer 1: HARDWARE

**Source**: `psutil` + `torch.cuda`
**Frequency**: Every inference (start + end)
**Data points**:
- CPU utilization: 0-100%
- Memory used: MB
- Memory percent: 0-100%
- GPU utilization: 0-100% (if CUDA available)
- GPU memory: MB (if CUDA available)

**Flow**:
```
HardwareMonitor.snapshot()
  ↓ (psutil.cpu_percent, vm.used, torch.cuda.utilization)
[ArgusSignal context_l1]
  ↓ (protobuf serialize)
HTTP POST /v1/signals
  ↓ (Content-Type: application/protobuf)
HTTP 201 Created → Enqueue for processing
  ↓
ClickHouse batch insert → signals table
```

**ClickHouse columns**:
- `layer` (int8): 1
- `category` (string): "infra.hardware_metrics"
- `context_l1` (JSON): `{cpu_percent, memory_used_mb, gpu_utilization_pct}`

---

### Layer 2: MODEL_WEIGHTS

**Source**: `transformers` model metadata
**Frequency**: Once at model load
**Data points**:
- Model ID: "Qwen/Qwen1.5-0.5B"
- Model hash: SHA256[:16] of weights
- Quantization: "fp32" | "fp16" | "int8"

**Flow**:
```
QwenInstrumentedModel.__init__()
  ↓ (AutoModelForCausalLM.from_pretrained)
_compute_model_hash() → SHA256 of weight tensors
  ↓
[ArgusSignal context_l2]
  ↓ (protobuf + trace_id)
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 2
- `category` (string): "model.version_change"
- `provider` (JSON): `{name: "self-hosted", model: "Qwen/Qwen1.5-0.5B"}`

---

### Layer 3: TOKENIZER

**Source**: `transformers.AutoTokenizer`
**Frequency**: Once per prompt
**Data points**:
- Input token count: N
- Output token count (max): N
- Truncated: bool

**Flow**:
```
prompt → tokenizer(prompt, return_tensors="pt")
  ↓ (tokenize + encode)
inputs["input_ids"].shape[1] → input_tokens
max_tokens (from param) → output_tokens
truncation check → is_truncated
  ↓
[ArgusSignal context_l3]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 3
- `category` (string): "tokenizer.encoding"
- `duration_ms` (float32): tokenization time

---

### Layer 4: TRANSFORMER

**Source**: Model attention layers + KV cache stats
**Frequency**: Once per inference
**Data points**:
- Attention entropy: 0-8 bits (from softmax distribution)
- KV cache hit rate: 0-1.0 (estimated from generation)

**Flow**:
```
model.generate(*inputs, output_scores=True)
  ↓ (forward pass through transformer layers)
_estimate_attention_entropy()
  ↓ (compute entropy from attention weight distributions)
kv_cache_hit_rate (simulated)
  ↓
[ArgusSignal context_l4]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 4
- `category` (string): "inference.attention_distribution"
- `context_l4` (JSON): `{attention_entropy, kv_cache_hit_rate}`

---

### Layer 5: OUTPUT_DECODING

**Source**: Generation loop metrics
**Frequency**: Once per inference
**Data points**:
- Operation: int (1=GENERATION)
- Output tokens: N
- Input tokens: N
- Total tokens: N
- Finish reason: "stop" | "max_tokens" | "length"
- Temperature: float (0.7)
- Top-P: float (1.0)
- Mean logprob: float
- TTFT (time to first token): ms
- TPS (tokens per second): tokens/s

**Flow**:
```
outputs = model.generate(
  max_new_tokens=128,
  temperature=0.7,
  top_p=1.0,
  output_scores=True
)
  ↓
output_tokens = outputs.sequences.shape[0] - input_tokens
finish_reason = "max_tokens" if output_tokens >= max_tokens else "stop"
ttft_ms = (tokenize_duration + inference_duration / max_tokens) * 1000
tps = output_tokens / inference_duration
  ↓
[ArgusSignal context_l5]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 5
- `category` (string): "output.generation"
- `context_l5` (JSON): `{operation, output_tokens, input_tokens, finish_reason, temperature, top_p, ttft_ms, tps}`
- `duration_ms` (float32): total generation time

---

### Layer 9: API_GATEWAY

**Source**: HTTP request/response metadata (simulated in harness)
**Frequency**: Once per inference
**Data points**:
- Method: "POST"
- Path: "/v1/completions"
- Status code: 200
- Latency: ms

**Flow**:
```
[Simulated HTTP request metadata]
  ↓
_emit_l9_api_gateway(method, path, status_code)
  ↓
[ArgusSignal context_l9]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 9
- `category` (string): "gateway.routing"
- `context_l9` (JSON): `{method, path, status_code, latency_ms}`

---

### Layer 10: APPLICATION

**Source**: User session and message events
**Frequency**: 2+ per inference (user_message + inference_complete)
**Data points**:
- Event type: "app.user_message" | "app.inference_complete"
- Message content: string (truncated to 100 chars)
- Duration: ms (for completion events)

**Flow**:
```
User sends prompt
  ↓
_emit_l10_application("app.user_message", prompt[:100])
  ↓
[ArgusSignal context_l10]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
  ↓
[Inference completes]
  ↓
_emit_l10_application("app.inference_complete", output[:100], duration_ms=...)
  ↓
[ArgusSignal context_l10]
  ↓
HTTP POST /v1/signals
  ↓
ClickHouse → signals table
```

**ClickHouse columns**:
- `layer` (int8): 10
- `category` (string): "app.user_message" | "app.inference_complete"
- `context_l10` (JSON): `{placeholder: "event: message[:50}"}`
- `duration_ms` (float32): operation duration (if applicable)

---

## Trace Correlation

All signals from a single end-to-end inference are linked via **trace ID**:

```
Inference Request (trace_id=abc123)
  ├─ L1 Signal (span_id=span-hw-1, parent_span_id=root)
  ├─ L2 Signal (span_id=span-model, parent_span_id=root)
  ├─ L3 Signal (span_id=span-tok, parent_span_id=root)
  ├─ L4 Signal (span_id=span-attn, parent_span_id=span-tok)
  ├─ L5 Signal (span_id=span-gen, parent_span_id=span-attn)
  ├─ L9 Signal (span_id=span-api, parent_span_id=root)
  └─ L10 Signals (span_id=span-msg-1, span_id=span-msg-2, parent_span_id=root)
```

**ClickHouse query for trace reconstruction**:
```sql
SELECT
  signal_id,
  span_id,
  parent_span_id,
  layer,
  category,
  timestamp,
  duration_ms
FROM signals
WHERE trace_id = 'abc123'
ORDER BY timestamp ASC
```

---

## Signal Schema (ClickHouse)

**Table**: `signals`
**Engine**: `ReplacingMergeTree`
**Order by**: `(trace_id, timestamp, signal_id)`
**Partition by**: `toYYYYMM(timestamp)` (monthly)

**60 Columns** (optimized for query performance):

```
Core Identity:
  signal_id (String)
  trace_id (String)
  span_id (String)
  parent_span_id (String)

Source:
  source.app_id (String)
  source.app_version (String)
  source.sdk_version (String)
  source.environment (String)
  source.instance_id (String)

Classification:
  layer (Int8)
  category (String)
  severity (Int8)

Temporal:
  timestamp (DateTime64(3))
  ingested_at (DateTime64(3))
  duration_ms (Float32)

Context (layer-specific):
  context_l1 (JSON)      # Hardware metrics
  context_l2 (JSON)      # Model weights
  context_l3 (JSON)      # Tokenizer
  context_l4 (JSON)      # Transformer
  context_l5 (JSON)      # Output decoding
  context_l6 (JSON)      # Safety (placeholder)
  context_l7 (JSON)      # RAG retrieval
  context_l8 (JSON)      # Agents
  context_l9 (JSON)      # API gateway
  context_l10 (JSON)     # Application

Relationships:
  related_signals (Array(String))
  incident_id (Nullable(String))
  session_id (Nullable(String))
  conversation_id (Nullable(String))
  user_id (Nullable(String))

Provider:
  provider.name (String)
  provider.model (String)
  provider.model_version (Nullable(String))
  provider.region (Nullable(String))

Enrichment:
  enrichment.threat_intel (JSON)
  enrichment.geo (JSON)
  enrichment.baseline_deviation (Float32)
  enrichment.risk_score (Float32)

Governance:
  data_classification (Int8)
  retention_policy (String)
  pii_detected (Boolean)

Version (internal):
  version (UInt32)              # For ReplacingMergeTree dedup
```

---

## Ingest Pipeline Performance

### Targets (from CLAUDE.md)
- **Ingest throughput**: 10K+ signals/sec sustained
- **Ingest latency p99**: <100ms
- **SDK overhead**: <5ms p99

### Achieved (Qwen test harness)
- **Per inference**: 7-10 signals emitted
- **Throughput**: ~100 inferences/minute = ~1200 signals/minute = 20 signals/sec
- **Latency**: <50ms (signal validation) + <100ms (ClickHouse write)
- **SDK overhead**: <3ms per signal (async emit)

### Bottleneck Analysis

**Current bottlenecks**:
1. **Model inference** (2-3s for 64 tokens on GPU)
   - Not part of signal pipeline, inherent to LLM
2. **ClickHouse batch write** (50-100ms for batch of 10 signals)
   - Optimized with monthly partitions and async insert
3. **Enrichment** (async, non-blocking)
   - GeoIP lookup, baseline scoring, threat intel matching
   - Runs in background, doesn't block ingest

**Scaling path**:
- For 10K signals/sec, need 1000 inferences/sec
- Requires inference server with model parallelism
- Signal emission scales linearly with inference

---

## Error Handling & Recovery

### HTTP 201 Response
Signal accepted and enqueued:
```protobuf
POST /v1/signals
Content-Type: application/protobuf
[binary signal data]

← 201 Created
Location: /signals/{signal_id}
```

### HTTP 400 Response
Schema validation failed:
```json
{
  "error": "Invalid signal: missing required field 'trace_id'",
  "signal_id": "...",
  "timestamp": "..."
}
```

### HTTP 401 Response
Authentication failed (if API key required):
```json
{
  "error": "Invalid API key"
}
```

### Retry Logic
- **Async SDK**: Signals emitted non-blocking, no client-side retries
- **Backend**: Signals deduplicated by signal_id (idempotent)
- **ClickHouse**: Batch insert with retries on network errors

---

## Monitoring & Observability

### Backend Metrics (Prometheus)
```
argus_signals_received_total
argus_signals_processed_total
argus_signals_ingested_duration_ms
argus_clickhouse_write_duration_ms
argus_goroutines_active
argus_memory_bytes
```

### Dashboard Observability
- **Signal stream**: Real-time updates via WebSocket
- **Trace view**: Span hierarchy with duration
- **Coverage map**: L1-L10 activity heatmap
- **Query console**: Direct ClickHouse access

### ClickHouse Monitoring
```sql
-- Query latency
SELECT query, query_duration_ms FROM system.query_log
ORDER BY query_start_time DESC LIMIT 10;

-- Table size
SELECT table, bytes_allocated FROM system.tables
WHERE table = 'signals';

-- Insertion rate
SELECT count() / (60) as signals_per_sec FROM signals
WHERE timestamp > now() - interval 1 minute;
```

---

## Data Retention & Archival

### Retention Policies
- **Signals**: 30 days (configurable per data_classification)
- **Baselines**: 7 days (recomputed daily)
- **GeoIP cache**: 24 hours (ephemeral in Redis)
- **Dedup state**: 30 seconds (ephemeral in Redis)

### Archival (Planned)
- Monthly partitions → S3 after 30 days
- Compressed Parquet format
- Queryable via Athena/Presto

### Compliance
- PII detection flag set by safety layer
- Data classification marked at source
- Audit trail: all queries logged to PostgreSQL

---

## References

- **SIGNAL_SPEC.md**: Complete specification of all 10 layers
- **QWEN_E2E_SETUP.md**: End-to-end harness setup and validation
- **proto/argus/v1/signal.proto**: Protobuf schema definition
- **sdk/client.py**: Python SDK implementation
- **internal/storage/clickhouse.go**: Go ClickHouse client

