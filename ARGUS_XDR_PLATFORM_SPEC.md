# Argus: Extended Detection & Response for LLM Systems

## Preamble — What This Document Is

This is a build specification for a production-grade observability and security platform that treats LLM-integrated systems the way modern XDR treats enterprise infrastructure — with full-stack signal coverage, correlated detection, investigation workflows, and response orchestration.

This is **not** a Python script collection glued together with Docker Compose. This is a platform specification that could be handed to a founding engineering team and produce a shippable product. Every section earns its presence by answering: *what signal are we capturing, why does it matter, and what does the operator do with it?*

---

## Part 1 — The Signal Surface

Before writing a single line of code, we define what we can observe. An XDR platform that doesn't enumerate its signal surface is just a log aggregator with a dashboard.

### 1.1 The LLM System Stack (Bottom-Up)

Every LLM-integrated system — regardless of provider, framework, or deployment model — touches these layers. Argus must have instrumentation points at each.

```
┌─────────────────────────────────────────────────────────┐
│  L10  APPLICATION / USER INTERFACE                      │
│       Sessions, conversations, user actions, feedback   │
├─────────────────────────────────────────────────────────┤
│  L9   API GATEWAY / SERVING                             │
│       Request routing, rate limits, auth, quota          │
├─────────────────────────────────────────────────────────┤
│  L8   ORCHESTRATION / AGENTS                            │
│       Tool calls, MCP, multi-step reasoning, planning   │
├─────────────────────────────────────────────────────────┤
│  L7   RAG / RETRIEVAL                                   │
│       Chunking, embedding, vector search, re-ranking    │
├─────────────────────────────────────────────────────────┤
│  L6   SAFETY & ALIGNMENT                                │
│       Input classifiers, output filters, policy engine  │
├─────────────────────────────────────────────────────────┤
│  L5   OUTPUT / DECODING                                 │
│       Token sampling, temperature, stop sequences       │
├─────────────────────────────────────────────────────────┤
│  L4   TRANSFORMER / INFERENCE                           │
│       Attention, KV cache, layer activations            │
├─────────────────────────────────────────────────────────┤
│  L3   TOKENIZER / EMBEDDINGS                            │
│       BPE encoding, vocab table, positional encoding    │
├─────────────────────────────────────────────────────────┤
│  L2   MODEL WEIGHTS / STORAGE                           │
│       SafeTensors, checkpoints, quantization, registry  │
├─────────────────────────────────────────────────────────┤
│  L1   HARDWARE / INFRASTRUCTURE                         │
│       GPU clusters, memory, orchestration, networking   │
└─────────────────────────────────────────────────────────┘
```

### 1.2 Signal Taxonomy — What Can Be Observed at Each Layer

Not every layer is equally accessible. Some layers are deep inside the model provider's stack (L1–L5 when using a hosted API). Others are fully under the operator's control (L7–L10 in a self-hosted RAG system). Argus must handle both cases: **deep instrumentation where available, proxy signals where not.**

#### L1 — Hardware / Infrastructure
**Direct signals** (self-hosted only):
- GPU utilization, memory pressure, thermal state, clock frequency
- Inter-node bandwidth saturation (NVLink/NVSwitch)
- Container/pod lifecycle events (OOM kills, restarts, evictions)
- Disk I/O for model loading, checkpoint reads

**Proxy signals** (API consumers):
- API response latency distribution (p50/p95/p99) — latency spikes correlate with provider-side resource contention
- Token throughput variance — degradation signals provider capacity issues
- Error rate patterns (429s, 500s, 503s) — provider health proxy

**Why it matters**: A hallucination spike that correlates with provider latency is a capacity issue, not a model issue. Without infrastructure signals, you misattribute.

#### L2 — Model Weights / Storage
**Direct signals** (self-hosted):
- Model version loaded, quantization level applied
- Weight checksum / integrity hash
- Hot-swap events (model version transitions under traffic)
- Weight loading latency, shard distribution across devices

**Proxy signals** (API consumers):
- Model identifier in API response headers/metadata
- Behavioral drift detected via output distribution shift (same prompts, different statistical profile over time)
- Provider model version change announcements (external feed)

**Why it matters**: Provider-side model updates without announcement are a real attack surface. Behavioral shift detection on stable prompts is a canary.

#### L3 — Tokenizer / Embeddings
**Direct signals**:
- Token count per request/response
- Token density anomalies (unusual ratio of special tokens to content tokens)
- Tokenizer version mismatch between encoding and model expectation
- Token-to-character ratio outliers (adversarial token stuffing)

**Proxy signals**:
- Input/output token counts from API billing metadata
- Prompt length vs response length ratios — anomalous ratios signal tokenizer-level manipulation

**Why it matters**: Token-level attacks (adversarial suffixes, token smuggling, homoglyph injection) happen here. If you can't see token distributions, you can't detect them.

#### L4 — Transformer / Inference
**Direct signals** (self-hosted with instrumentation):
- Attention weight distributions per layer (which tokens attend to what)
- KV cache utilization, eviction events
- Activation magnitudes per layer (anomalous activations correlate with adversarial inputs)
- Inference time per layer (layer-level profiling)
- Logit distributions pre-sampling

**Proxy signals** (API consumers):
- Time-to-first-token (TTFT) — proxy for prefill compute, correlates with prompt complexity
- Inter-token latency variance — sudden changes suggest different inference paths
- Total inference time vs token count — non-linear relationship signals unusual processing

**Why it matters**: This is where the model "thinks." Attention hijacking, activation injection, and reasoning manipulation all leave traces here. Most platforms skip this entirely.

#### L5 — Output / Decoding
**Direct signals**:
- Per-token log probabilities (the single most valuable signal for hallucination detection)
- Cumulative sequence probability
- Token-level entropy (high entropy = model uncertainty)
- Sampling parameters applied (temperature, top-p, top-k)
- Stop reason (natural stop, max tokens, stop sequence hit)
- Speculative decoding acceptance rate

**Proxy signals**:
- Log probabilities from API (where exposed — OpenAI, some others)
- Response truncation patterns
- Finish reason metadata

**Why it matters**: Hallucination has a measurable signature in token-level confidence. Without logprobs, you're guessing. This is the most neglected signal source in the current LLM observability ecosystem.

#### L6 — Safety & Alignment
**Direct signals**:
- Input classifier scores (injection probability, toxicity, jailbreak confidence)
- Output filter triggers (PII detected, policy violation, content category)
- Safety layer latency (adds to total response time)
- Safety bypass attempts (inputs that passed classifiers but triggered output filters)
- Policy version applied

**Proxy signals**:
- Refusal patterns in model output (model-level safety triggering)
- Content moderation API scores (if using external moderation)
- Refusal-to-completion ratio over time

**Why it matters**: This layer is the first line of defense and the most actively attacked. Monitoring its efficacy — not just its presence — is the difference between "we have guardrails" and "we know our guardrails work."

#### L7 — RAG / Retrieval
**Direct signals** (this layer is almost always under operator control):
- Query embedding vector (for drift detection over time)
- Retrieval latency per query
- Number of chunks retrieved, re-ranked, injected into context
- Similarity scores of retrieved chunks (relevance distribution)
- Re-ranker score distribution (are re-ranked results significantly different from raw retrieval?)
- Chunk metadata: source document, chunk position, document freshness
- Context window utilization (% of context consumed by retrieved chunks vs. prompt vs. history)
- Embedding model version, dimensionality
- Vector index health (fragmentation, index size, recall at N)

**Derived signals**:
- Retrieval relevance score vs. generation groundedness — measures whether the model actually used what was retrieved
- Chunk poisoning indicators — statistically anomalous chunks appearing in retrieval that don't match the corpus distribution
- Embedding drift — same queries returning different chunks over time without corpus changes
- Citation accuracy — does the model's cited source match the chunk that was actually retrieved?

**Why it matters**: RAG is the most common production LLM architecture. It's also the most porous — data poisoning, chunk injection, embedding manipulation, and retrieval evasion are all active threat vectors. This is where Argus earns its XDR positioning.

#### L8 — Orchestration / Agents
**Direct signals**:
- Tool call sequence (ordered list of tools invoked per agent turn)
- Tool call arguments (parameters passed to each tool)
- Tool call results (return values, errors, timeouts)
- Agent planning steps (if using chain-of-thought or planning frameworks)
- MCP server connections (which external services are connected, schema discovery events)
- Multi-step loop count (how many think→act→observe cycles before termination)
- Permission scope at each tool call (what the agent was authorized to do vs. what it attempted)
- Inter-agent communication (in multi-agent systems: message passing, delegation events)

**Derived signals**:
- Tool call divergence from baseline (agent calling tools it has never called before)
- Privilege escalation patterns (agent requesting broader permissions mid-conversation)
- Runaway loop detection (cycle count exceeding historical baselines)
- Cross-tool data flow (sensitive data from one tool being passed to another — PII from email into web search)

**Why it matters**: Agents are the highest-risk LLM deployment pattern. An agent with tool access is an autonomous actor. Without full observability into its decision chain, you cannot reconstruct what happened, why, or whether it was authorized.

#### L9 — API Gateway / Serving
**Direct signals**:
- Request/response headers, status codes, content types
- Authentication events (JWT validation, API key usage, scope)
- Rate limit state (tokens remaining, throttle events)
- Request routing decisions (which model, which replica)
- Request queuing time
- Billing metadata (tokens consumed, cost per request)

**Derived signals**:
- API key behavioral profiling (usage patterns per key)
- Cost anomaly detection (sudden spend increases)
- Request pattern anomalies (unusual model/endpoint combinations)

**Why it matters**: This is the perimeter. Stolen API keys, credential stuffing, and cost attacks are financially damaging threats that many teams only discover on the invoice.

#### L10 — Application / User Interface
**Direct signals**:
- Session events (start, end, duration, page navigation)
- User actions (message sent, file uploaded, feedback given, copy/paste)
- Conversation metadata (turn count, topic drift, language switching)
- User feedback signals (thumbs up/down, regeneration requests, edits)
- Client-side errors (rendering failures, timeout displays)

**Derived signals**:
- User satisfaction proxy (feedback + engagement pattern)
- Conversation coherence score (is the conversation degrading?)
- Abuse pattern detection (high-volume automated submissions, prompt injection testing patterns)
- Multi-session attack correlation (distributed attack across sessions)

**Why it matters**: This is where you see intent. A prompt injection attempt is a signal at L6; the user who submitted it, their session history, and their behavioral profile are signals at L10. Without this layer, every attack is anonymous.

---

### 1.3 Signal Coverage Matrix

The honest truth about what Argus can observe depends on deployment context:

```
                        Self-Hosted    API Consumer    Hybrid
Layer                   (full access)  (proxy only)    (RAG+API)
─────────────────────   ────────────   ─────────────   ─────────
L1  Hardware            ████████████   ░░░░░░░░░░░░   ░░░░░░░░░
L2  Model Weights       ████████████   ░░░░░░░░░░░░   ░░░░░░░░░
L3  Tokenizer           ████████████   ▓▓▓▓▓▓░░░░░░   ▓▓▓▓▓▓░░░
L4  Transformer         ████████████   ░░░░░░░░░░░░   ░░░░░░░░░
L5  Output/Decoding     ████████████   ▓▓▓▓▓▓▓▓░░░░   ▓▓▓▓▓▓▓▓░
L6  Safety/Alignment    ████████████   ▓▓▓▓░░░░░░░░   ▓▓▓▓▓▓▓░░
L7  RAG/Retrieval       ████████████   N/A            ████████████
L8  Agents/Tools        ████████████   ████████████   ████████████
L9  API Gateway         ████████████   ▓▓▓▓▓▓▓▓▓▓▓▓   ▓▓▓▓▓▓▓▓▓▓
L10 Application         ████████████   ████████████   ████████████

████ = full direct signals    ▓▓▓▓ = partial/proxy    ░░░░ = minimal/none
```

**Design principle**: Argus never pretends to have visibility it doesn't have. The platform exposes its own coverage gaps to the operator. If a layer is not instrumented, the dashboard shows it as a blind spot, not an absence of threats.

---

## Part 2 — Threat Model

### 2.1 Threat Categories Mapped to Layers

An XDR without a threat model is a monitoring dashboard. Argus maps known and emerging threats to specific layers and defines which signals are required to detect each.

| Threat | Primary Layer(s) | Required Signals | Detection Approach |
|--------|-----------------|------------------|--------------------|
| **Prompt injection (direct)** | L3, L6, L10 | Input text, classifier score, token distribution | Pattern match + classifier score + token entropy analysis |
| **Prompt injection (indirect)** | L7, L8 | Retrieved chunk content, tool results, source metadata | Content analysis of retrieved/tool data before context injection |
| **Jailbreak / safety bypass** | L6, L5, L10 | Classifier scores, output content, refusal patterns | Classifier evasion detection + output policy scan |
| **Hallucination** | L5, L7 | Logprobs, retrieval relevance, grounding score | Token-level confidence vs. retrieval grounding correlation |
| **Data poisoning (RAG)** | L7 | Chunk metadata, embedding distribution, ingestion events | Statistical anomaly in chunk population + embedding space drift |
| **Data exfiltration** | L8, L5, L10 | Tool call arguments, output content, session context | PII/sensitive data flow tracking across tool boundaries |
| **Model extraction** | L9, L4 | API request patterns, logprob requests, systematic probing | Behavioral profiling of API key usage patterns |
| **Denial of wallet** | L9, L3 | Token counts, API costs, request rates | Cost anomaly detection + rate pattern analysis |
| **Agent hijacking** | L8 | Tool call sequence, permission scope, planning steps | Deviation from authorized tool call patterns |
| **Privilege escalation** | L8, L9 | Permission requests, scope changes, auth events | Progressive scope expansion detection |
| **Embedding inversion** | L7, L3 | Embedding vectors, query patterns | Access pattern anomaly on embedding endpoints |
| **Supply chain (model swap)** | L2, L5 | Model version, behavioral fingerprint, output distribution | Behavioral consistency monitoring against known baseline |
| **Reasoning manipulation** | L4, L8 | Chain-of-thought content, planning steps, decision trace | Logical consistency analysis of reasoning chains |
| **Multi-turn social engineering** | L10, L6 | Conversation history, topic drift, escalation patterns | Session-level behavioral analysis across turns |
| **Tool result injection** | L8 | Tool results, tool source validation, result schema | Integrity validation of tool return values |

### 2.2 Threat Coverage Philosophy

Argus acknowledges three tiers of detection capability:

1. **Deterministic detection** — Pattern matching, policy rules, schema validation. High precision, narrow coverage. Works today.
2. **Statistical detection** — Baseline deviation, distribution shift, anomaly scoring. Medium precision, broad coverage. Requires training period.
3. **Semantic detection** — Content analysis, reasoning chain validation, cross-signal correlation. Variable precision, requires compute. Uses LLM-as-judge or specialized models.

Every detection rule in Argus declares which tier it operates at, its expected false positive rate, and its signal dependencies. No black-box detectors.

---

## Part 3 — The Unified Signal Schema

### 3.1 Core Signal Envelope

Every observation in Argus — regardless of source layer, deployment model, or provider — is normalized into a single envelope. This is the atom of the platform.

```
ArgusSignal {
  // Identity
  signal_id         : ULID                    // Universally unique, time-sortable
  trace_id          : string                  // Distributed trace correlation
  span_id           : string                  // Span within trace (for nested operations)
  parent_span_id    : string | null           // Parent span (agent step → tool call → result)

  // Source
  source {
    app_id          : string                  // Registered application identifier
    app_version     : semver                  // Application version
    sdk_version     : semver                  // Argus SDK version
    environment     : enum[dev|staging|prod]  // Deployment environment
    instance_id     : string                  // Specific runtime instance
  }

  // Classification
  layer             : enum[L1..L10]           // Stack layer (from taxonomy)
  category          : string                  // Signal category (e.g., "retrieval.search", "agent.tool_call", "safety.classifier")
  severity          : enum[info|low|medium|high|critical]

  // Temporal
  timestamp         : ISO8601 (nanosecond)    // When the event occurred
  duration_ms       : float | null            // How long the operation took
  ingested_at       : ISO8601                 // When Argus received it

  // Context (layer-specific payload)
  context           : map<string, any> {
    // Structured fields vary by layer and category
    // See section 3.2 for per-layer schemas
  }

  // Relationships
  related_signals   : [signal_id]             // Correlated signals (populated by pipeline)
  incident_id       : string | null           // If part of an incident
  session_id        : string | null           // User session
  conversation_id   : string | null           // Conversation/thread
  user_id           : string | null           // End user (if known, hashed)

  // Provider metadata
  provider {
    name            : string                  // "openai" | "anthropic" | "bedrock" | "self-hosted" | ...
    model           : string                  // Model identifier
    model_version   : string | null           // Specific version if known
    region          : string | null           // Deployment region
  }

  // Enrichment (populated by pipeline, not SDK)
  enrichment {
    threat_intel    : [match] | null          // TI matches
    geo             : geo_data | null         // Geo-IP if applicable
    baseline_deviation : float | null         // Z-score from baseline
    risk_score      : float | null            // Composite risk score (0.0–1.0)
  }

  // Governance
  data_classification : enum[public|internal|confidential|restricted]
  retention_policy    : string                // Which retention rule applies
  pii_detected        : boolean               // Whether PII scanning flagged content
}
```

### 3.2 Per-Layer Context Schemas

Each layer defines its own structured context fields. This is what goes inside `context {}`.

**L7 — RAG/Retrieval context example:**
```
context {
  operation         : "vector_search" | "rerank" | "chunk_inject" | "embedding"
  query_text        : string (truncated/hashed per policy)
  query_embedding   : float[] | null (optional, for drift analysis)
  results_count     : int
  results_scores    : float[]           // Similarity scores of returned chunks
  top_chunk_source  : string            // Document source of highest-ranked chunk
  context_window_pct: float             // % of context window consumed by retrieval
  embedding_model   : string
  vector_index      : string            // Which index was queried
  reranker_applied  : boolean
  reranker_delta    : float | null      // Score change from reranking
  latency_breakdown : {
    embedding_ms    : float
    search_ms       : float
    rerank_ms       : float
  }
}
```

**L8 — Agent/Tool context example:**
```
context {
  operation         : "tool_call" | "tool_result" | "planning" | "delegation"
  tool_name         : string
  tool_provider     : string            // "mcp:gmail" | "function" | "builtin"
  tool_arguments    : map<string, any>  // Arguments passed (redacted per policy)
  tool_result       : string (truncated)
  tool_latency_ms   : float
  tool_error        : string | null
  step_number       : int               // Which step in agent loop
  total_steps       : int | null        // Total steps if known
  permissions_used  : [string]          // What permissions the tool invoked
  permissions_requested : [string] | null
  data_flow_tags    : [string]          // Tags for data flowing through (e.g., "contains_pii", "external_source")
}
```

**L5 — Output/Decoding context example:**
```
context {
  operation         : "generation" | "streaming"
  output_tokens     : int
  input_tokens      : int
  total_tokens      : int
  finish_reason     : "stop" | "max_tokens" | "tool_use" | "content_filter"
  temperature       : float
  top_p             : float
  logprobs          : [{token, logprob, top_alternatives}] | null
  mean_logprob      : float | null      // Average confidence across output
  min_logprob       : float | null      // Lowest confidence token
  entropy_mean      : float | null      // Mean entropy across output
  entropy_variance  : float | null
  ttft_ms           : float             // Time to first token
  tps               : float             // Tokens per second
}
```

Every layer has a defined schema. SDKs validate against these schemas before sending. The instance rejects signals that don't conform. No unstructured log dumping.

---

## Part 4 — Platform Architecture

### 4.1 Design Principles

1. **Separation of collection from analysis.** The SDK collects and ships. The instance stores, correlates, and detects. The decision engine (Kairos) evaluates policy. These are independent failure domains.

2. **Wire protocol, not library coupling.** SDKs communicate over a defined wire protocol (protobuf over gRPC for high throughput, JSON over HTTP for simplicity). Any language can implement an SDK. Any system can emit signals.

3. **Storage is the platform.** The choice of storage engine determines query capability, retention cost, and scale ceiling. This is not an afterthought.

4. **Detection is configuration, not code.** Detection rules are data (YAML/JSON), not compiled code. Operators create, modify, and version detection rules without redeploying the platform.

5. **The dashboard is the investigation tool, not a pretty chart.** Every visualization exists to answer a specific operator question. No vanity metrics.

### 4.2 Component Architecture

```
                   ┌──────────────────────────────────────────────────┐
                   │               INSTRUMENTED APPLICATIONS          │
                   │                                                  │
                   │  App A (RAG)    App B (Agent)    App C (Chat)    │
                   │    │               │                │            │
                   │    └── Argus SDK ──┴── Argus SDK ──┘            │
                   └────────────┬─────────────┬──────────────────────┘
                                │ gRPC/HTTP   │ OTLP
                                ▼             ▼
┌───────────────────────────────────────────────────────────────────────┐
│                        ARGUS INSTANCE                                 │
│                                                                       │
│  ┌─────────────┐   ┌──────────────┐   ┌────────────────────────┐     │
│  │  INGEST     │   │  PROCESS     │   │  STORE                 │     │
│  │             │   │              │   │                        │     │
│  │ • gRPC rx   │──▶│ • Validate   │──▶│ • Signals (columnar)   │     │
│  │ • HTTP rx   │   │ • Normalize  │   │ • Config  (relational) │     │
│  │ • OTLP rx   │   │ • Enrich     │   │ • State   (KV cache)   │     │
│  │ • Syslog rx │   │ • Correlate  │   │ • Blobs   (object)     │     │
│  │             │   │ • Route      │   │                        │     │
│  └─────────────┘   └──────┬───────┘   └────────────┬───────────┘     │
│                           │                        │                 │
│                           ▼                        │                 │
│  ┌──────────────────────────────────┐              │                 │
│  │  DETECT                          │◀─────────────┘                 │
│  │                                  │                                │
│  │ • Rule engine (deterministic)    │                                │
│  │ • Baseline engine (statistical)  │                                │
│  │ • Correlation engine (temporal)  │                                │
│  │ • Semantic engine (LLM-judge)    │                                │
│  └──────────────┬───────────────────┘                                │
│                 │                                                    │
│                 ▼                                                    │
│  ┌──────────────────────────────────┐   ┌─────────────────────┐     │
│  │  RESPOND                         │   │  SERVE              │     │
│  │                                  │   │                     │     │
│  │ • Alert routing                  │   │ • Query API         │     │
│  │ • Incident lifecycle             │   │ • WebSocket (live)  │     │
│  │ • Notification dispatch          │   │ • Dashboard SPA     │     │
│  │ • Webhook delivery               │   │ • Config API        │     │
│  │ • Kairos ECL integration ───────────▶│ • Decision audit    │     │
│  └──────────────────────────────────┘   └─────────────────────┘     │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘
                                                    │
                                          ┌─────────▼──────────┐
                                          │  KAIROS ECL        │
                                          │  (Optional Sidecar)│
                                          │                    │
                                          │  Policy evaluation │
                                          │  Decision logging  │
                                          │  Observable output │
                                          └────────────────────┘
```

### 4.3 Technology Choices — And Why

This section doesn't list options. It makes decisions and defends them.

#### Core Runtime: Go

**Why not Python**: Python is adequate for prototyping and ML. It is inadequate for a high-throughput signal ingestion system. The GIL limits true concurrency. `asyncio` adds complexity without solving the fundamental problem. Every production observability platform (Prometheus, Grafana Agent, Vector, Telegraf, OpenTelemetry Collector) is written in Go or Rust for a reason: predictable latency under load, efficient memory usage, and native concurrency.

**Why not Rust**: Rust produces faster binaries but has a steeper contributor curve. Go hits the right tradeoff for a platform that needs community contribution.

**Decision**: Core ingestion, processing, detection, and API services in Go. ML/semantic detection components (which need Python ML libraries) run as isolated gRPC services callable from the Go core.

#### Wire Protocol: Protocol Buffers + gRPC (primary), HTTP/JSON (convenience)

- gRPC for high-throughput SDK→Instance communication. Streaming support. Schema enforcement at the protocol level. ~10x smaller payloads than JSON.
- HTTP/JSON as a convenience layer for integration, debugging, and SDKs in languages where gRPC support is weak.
- OpenTelemetry Protocol (OTLP) as a native receiver — any OTEL-instrumented system can ship signals to Argus without an Argus SDK.

#### Signal Storage: ClickHouse

**Why not OpenSearch/Elasticsearch**: These are full-text search engines repurposed as time-series stores. They work for log search but are expensive for high-cardinality columnar queries ("give me p99 latency by model, by layer, by hour for the last 30 days"). They consume 5–10x more memory per GB stored.

**Why not PostgreSQL with TimescaleDB**: Good for lower volumes. Crumbles at 10K+ signals/sec sustained. Not designed for the query patterns of observability (wide scans, time-range aggregations, high-cardinality GROUP BY).

**Why ClickHouse**: Purpose-built for exactly this workload. Columnar storage means aggregation queries are fast. Compression ratios of 10–20x on structured signal data. Handles 100K+ inserts/sec on modest hardware. Built-in materialized views for pre-aggregation. SQL interface means operators can write ad-hoc queries without learning a DSL.

**Decision**: ClickHouse for signal storage and analytics. PostgreSQL for configuration, user management, and incident state (relational data that needs ACID). Redis for ephemeral state (rate limiting, deduplication windows, real-time counters).

#### Dashboard: React + TypeScript

No framework debate. React has the largest ecosystem for data visualization, the best component library support (Shadcn/ui), and the deepest talent pool. TypeScript for type safety on complex data structures.

Charting: Apache ECharts (not Plotly — ECharts handles 100K+ data points without degradation, has better time-series support, and renders to canvas for performance). D3 for custom visualizations that ECharts can't handle.

Real-time: WebSocket for live signal streams. Server-Sent Events as fallback.

### 4.4 Deployment Models

Argus ships as **statically compiled binaries** and **container images**. Not Python wheels. Not npm packages.

#### Single Binary (Development / Small Scale)

```
$ argus server --config argus.yaml
```

One binary. Embeds ingestion, processing, detection, API, and dashboard. Connects to external ClickHouse and PostgreSQL (or uses embedded SQLite for config + embedded ClickHouse-local for signals in dev mode).

This is how an individual developer or small team evaluates Argus. No Docker required. No Kubernetes. Download. Run. Connect SDK. See signals.

#### Container (Production / Medium Scale)

```yaml
# docker-compose.yml — production stack
services:
  argus-ingest:
    image: ghcr.io/argus-security/argus:latest
    command: ["argus", "ingest"]
    ports: ["4317:4317", "8080:8080"]      # gRPC, HTTP

  argus-detect:
    image: ghcr.io/argus-security/argus:latest
    command: ["argus", "detect"]

  argus-api:
    image: ghcr.io/argus-security/argus:latest
    command: ["argus", "api"]
    ports: ["9090:9090"]

  argus-dashboard:
    image: ghcr.io/argus-security/argus-dashboard:latest
    ports: ["3000:3000"]

  clickhouse:
    image: clickhouse/clickhouse-server:latest

  postgres:
    image: postgres:16

  redis:
    image: redis:7-alpine
```

Same binary, different `command` argument. Each mode runs only the relevant subsystem. Horizontal scaling via replicas.

#### Kubernetes (Enterprise / Large Scale)

Helm chart with:
- HPA on ingestion pods (scale on signal queue depth)
- Separate node pools for detection (CPU-intensive) vs. ingestion (I/O-intensive)
- ClickHouse Operator for cluster management
- Network policies for isolation
- RBAC-integrated service accounts

---

## Part 5 — Detection Engine Specification

### 5.1 Detection Rule Format

Detection rules are YAML documents. They declare inputs, conditions, and outputs. They are versioned, testable, and reviewable like code.

```yaml
# rules/rag/chunk-poisoning.yaml
apiVersion: argus/v1
kind: DetectionRule
metadata:
  id: rag-003
  name: Anomalous chunk injection in retrieval results
  description: >
    Detects chunks appearing in retrieval results that are statistically
    anomalous relative to the corpus baseline — potential data poisoning.
  author: security-team
  version: 1.2.0
  created: 2025-06-01
  updated: 2025-09-15
  tags: [rag, data-poisoning, integrity]
  mitre_atlas: AML.T0020  # Data poisoning

spec:
  # What layers and signal categories this rule consumes
  inputs:
    - layer: L7
      category: retrieval.search
      fields: [results_scores, top_chunk_source, results_count]

  # Detection tier
  tier: statistical  # deterministic | statistical | semantic

  # Conditions
  conditions:
    all:
      - field: context.results_count
        operator: gte
        value: 1
      - field: enrichment.baseline_deviation
        # This field is computed by the baseline engine
        # comparing current retrieval score distribution
        # against the 7-day rolling baseline for this app + index
        operator: gte
        value: 3.0  # 3-sigma deviation
      - field: context.results_scores[0]
        # Top result has anomalously high similarity
        operator: gte
        value: 0.99

  # What to emit when conditions match
  output:
    severity: high
    confidence_formula: "min(1.0, baseline_deviation / 5.0)"
    alert: true
    incident_correlation:
      group_by: [source.app_id, context.vector_index]
      window: 30m
    tags: [data-poisoning, rag-integrity]

  # Baseline requirements (for statistical tier)
  baseline:
    metric: context.results_scores
    aggregation: distribution  # mean | percentile | distribution
    window: 7d
    min_samples: 1000
    group_by: [source.app_id, context.vector_index]

  # Testing
  test_cases:
    - name: "Normal retrieval — should not fire"
      input:
        context: { results_count: 5, results_scores: [0.85, 0.82, 0.79, 0.71, 0.68] }
        enrichment: { baseline_deviation: 0.5 }
      expected: no_detection

    - name: "Poisoned chunk — should fire"
      input:
        context: { results_count: 5, results_scores: [0.995, 0.82, 0.79, 0.71, 0.68] }
        enrichment: { baseline_deviation: 4.2 }
      expected:
        severity: high
        confidence: 0.84
```

### 5.2 Detection Tiers (Expanded)

**Tier 1 — Deterministic**: Regex, pattern match, threshold comparison, schema validation. Runs inline in the processing pipeline. Sub-millisecond. Zero false negatives on exact matches.

**Tier 2 — Statistical**: Baseline deviation (z-score), distribution shift (KL divergence), rate anomaly (sliding window counters), cardinality anomaly. Requires a training window. Runs as a separate pipeline stage. 1–10ms.

**Tier 3 — Temporal Correlation**: Multi-signal pattern matching over time windows. "Signal A at L6 followed by signal B at L8 within 60 seconds on the same trace_id." Runs in the correlation engine. 10–50ms.

**Tier 4 — Semantic**: LLM-as-judge evaluation. "Does this output contradict the retrieved context?" Requires inference call to a classifier model (can be local small model or API). 100–2000ms. Used selectively, not on every signal.

### 5.3 Baseline Engine

The baseline engine continuously computes statistical profiles for every combination of:
- `(app_id, layer, category)` — per-app, per-layer baselines
- `(app_id, provider.model)` — per-model baselines
- `(app_id, user_id)` — per-user baselines (for behavioral profiling)

Profiles include: mean, stddev, percentiles (p50/p95/p99), distribution histogram, rate (events/minute), and cardinality (distinct values).

Baselines have a **learning period** (configurable, default 7 days) during which detections are suppressed and the system builds its statistical model. After the learning period, deviations are scored and surfaced.

---

## Part 6 — Dashboard & Operator Experience

### 6.1 Dashboard Philosophy

The dashboard exists to answer questions, not to display charts. Every view is designed around an operator workflow.

### 6.2 Core Views

**1. Coverage Map**
The first thing an operator sees. A visual representation of the LLM stack (L1–L10) with real-time overlay showing which layers have active instrumentation, which have signals flowing, and which are blind spots.

- Green: Signals flowing, detectors active
- Yellow: Signals flowing, no detectors configured
- Gray: No instrumentation at this layer
- Red: Instrumentation present but signals stopped (outage or SDK failure)

This is the single most important view. It answers: "Can I actually see what's happening in my LLM system?"

**2. Signal Stream (Live)**
Chronological feed of signals as they arrive. Filterable by layer, category, severity, app, user, provider. Each signal expandable to show full context.

Not a log dump. Each signal row shows:
- Timestamp, layer badge, category, severity indicator
- One-line summary (auto-generated from context fields)
- Trace ID (clickable to see full trace)
- Detection status (was a rule triggered? which one?)

**3. Trace View**
Given a trace_id, render the full lifecycle of a request through the LLM system. This is the investigation view.

```
Trace: trc-a8f3b2...
Duration: 2,340ms

L10 Application     ──[user message received]────────────────────────────┐
                                                                         │
L9  API Gateway      ──[auth OK, routed to gpt-4o]───────────────────┐  │
                                                                      │  │
L6  Safety           ──[input classifier: injection=0.12, pass]───┐  │  │
                                                                   │  │  │
L7  RAG/Retrieval    ──[query embedded]──[5 chunks retrieved]──┐  │  │  │
                                                                │  │  │  │
L4  Transformer      ──[inference: 1,847ms]────────────────┐   │  │  │  │
                                                            │   │  │  │  │
L5  Output           ──[312 tokens, mean_logprob=-0.42]─┐  │   │  │  │  │
                                                         │  │   │  │  │  │
L6  Safety           ──[output filter: PII=0.01, pass]  │  │   │  │  │  │
                                                         │  │   │  │  │  │
L8  Agents           ──[tool_call: search_docs]──────── │  │   │  │  │  │
                     ──[tool_result: 3 results]          │  │   │  │  │  │
                                                         │  │   │  │  │  │
L10 Application      ──[response delivered to user]──────┘──┘───┘──┘──┘──┘

⚠ Detection: rag-003 triggered (baseline_deviation=3.2, confidence=0.64)
   → Alert routed to #llm-security Slack channel
```

**4. Detection Dashboard**
Performance metrics for every detection rule. Per rule:
- Total evaluations, true positives, false positives
- Precision, recall, F1 over time
- Signal-to-alert conversion rate
- Average detection latency
- Baseline status (learning / active / stale)

Operators use this to tune thresholds, disable noisy rules, and identify coverage gaps.

**5. Incident Timeline**
When multiple signals correlate into an incident, render them as a timeline visualization. X-axis is time. Y-axis is stack layer. Each dot is a signal, colored by severity, sized by confidence. Connecting lines show trace relationships.

This is how an operator reconstructs an attack. "First, an unusual chunk appeared in retrieval (L7). Then the model's output confidence dropped (L5). Then the agent made an unauthorized tool call (L8). All within the same conversation, 45 seconds apart."

**6. Configuration Editor**
- Detection rules: YAML editor with syntax validation, test runner, dry-run mode
- Alert routing: Visual builder for condition → channel mapping
- Data retention: Per-layer, per-severity retention policies
- SDK registration: API key management, app registration, SDK health
- User management: RBAC (admin, analyst, viewer)

**7. Threat Hunting / Query Interface**
SQL-like query interface against ClickHouse. Operators write ad-hoc queries for investigation.

```sql
-- Find all tool calls that passed PII across tool boundaries
SELECT
  trace_id,
  context.tool_name,
  context.data_flow_tags,
  timestamp
FROM signals
WHERE layer = 'L8'
  AND category = 'agent.tool_call'
  AND has(context.data_flow_tags, 'contains_pii')
  AND timestamp > now() - INTERVAL 24 HOUR
ORDER BY timestamp DESC
```

Saved queries become reusable hunt playbooks. Successful hunts can be converted to detection rules.

---

## Part 7 — SDK Specification

### 7.1 Design Principles

1. **Zero-performance-impact default.** Signal collection is async, batched, and non-blocking. If the Argus Instance is unreachable, the application is unaffected.

2. **Explicit instrumentation, not magic.** No monkey-patching, no auto-instrumentation of arbitrary libraries. The developer decides what to instrument. Argus provides clean APIs to make that easy.

3. **Schema-enforced signals.** SDKs validate signals against per-layer schemas before sending. Malformed signals are rejected locally with a debug log, never shipped.

4. **Provider-agnostic.** The SDK doesn't know or care whether the instrumented application uses OpenAI, Anthropic, Bedrock, a local model, or all of the above. It captures signals about what the application does, not how the model works.

### 7.2 SDK API (Python — Reference Implementation)

```python
from argus_sdk import Argus, Layer, Category

# Initialize once per application process
argus = Argus(
    endpoint="grpc://argus.internal:4317",  # or http://...
    app_id="my-rag-app",
    environment="production",
    api_key=os.environ["ARGUS_API_KEY"],
    # Optional: batch config
    batch_size=200,          # signals per batch
    flush_interval_ms=3000,  # max wait before flush
    max_queue_size=10000,    # backpressure limit
)

# --- Decorator pattern (simple cases) ---

@argus.observe(Layer.L7, Category.RETRIEVAL_SEARCH)
def search_documents(query: str) -> list[Document]:
    """SDK automatically captures:
    - duration_ms (measured)
    - function arguments (if configured)
    - return value metadata (count, types)
    - exceptions (if raised)
    """
    return vector_db.search(query, top_k=5)


# --- Context manager pattern (structured context) ---

async def generate_response(prompt: str, retrieved_chunks: list):
    with argus.span(Layer.L5, Category.OUTPUT_GENERATION) as span:
        response = await llm.generate(prompt, context=retrieved_chunks)

        # Add structured context matching L5 schema
        span.set_context({
            "output_tokens": response.usage.completion_tokens,
            "input_tokens": response.usage.prompt_tokens,
            "finish_reason": response.finish_reason,
            "temperature": 0.7,
            "mean_logprob": compute_mean_logprob(response.logprobs),
            "ttft_ms": response.metrics.time_to_first_token,
        })

        # Optional: set severity based on your own logic
        if response.finish_reason == "content_filter":
            span.set_severity("medium")

    return response


# --- Manual signal emission (maximum control) ---

argus.emit(
    layer=Layer.L8,
    category=Category.AGENT_TOOL_CALL,
    severity="info",
    context={
        "tool_name": "send_email",
        "tool_provider": "mcp:gmail",
        "tool_arguments": {"to": "[redacted]", "subject": "Q3 report"},
        "permissions_used": ["gmail.send"],
        "step_number": 3,
        "data_flow_tags": ["contains_pii"],
    },
    trace_id=current_trace_id,
    span_id=generate_span_id(),
    parent_span_id=agent_loop_span_id,
)


# --- Provider-specific wrappers (optional convenience) ---

from argus_sdk.providers import openai_wrapper

# Wraps the OpenAI client to automatically emit L5 signals
# on every completion call. Zero code changes to existing calls.
client = openai_wrapper(openai.Client(), argus)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[...],
)
# → Automatically emits L5 signal with token counts, latency,
#   logprobs (if requested), finish reason, model version
```

### 7.3 SDK Implementations Required

| Language   | Pattern         | Priority | Rationale |
|-----------|----------------|----------|-----------|
| Python     | Decorator, context manager, manual | P0 | Most LLM applications are Python |
| TypeScript | Middleware, manual | P0 | API backends, serverless functions |
| Go         | Hook, manual | P1 | High-performance services, infrastructure |
| Java/Kotlin | Annotation, manual | P2 | Enterprise environments |
| Rust       | Trait, manual | P3 | Performance-critical inference servers |

All SDKs share the same protobuf schema for signals. All communicate over the same wire protocol. An application can use multiple SDKs (Python for ML code, Go for the API layer) and all signals correlate via trace_id.

### 7.4 OpenTelemetry Bridge

For teams already using OTEL, Argus accepts OTLP natively. An OTEL-to-Argus mapping layer converts OTEL spans/metrics into Argus signals. This means Argus can ingest signals from any OTEL-instrumented system without requiring an Argus SDK.

The mapping:
- OTEL span → Argus signal (with layer inferred from span attributes)
- OTEL span attributes → Argus context fields
- OTEL trace_id → Argus trace_id (direct passthrough)

This makes Argus composable with existing observability stacks. You don't have to rip out DataDog or Jaeger. Argus sits alongside them, consuming the same telemetry but applying LLM-specific detection.

---

## Part 8 — Kairos ECL Integration

Kairos is not a component of Argus. It is a consumer of Argus signals and a producer of observable decisions that Argus stores and displays.

### 8.1 Integration Pattern

```
Argus Detection Engine
    │
    │  (detection event: signals aggregated, risk scored)
    │
    ├──▶ Alert routing (always)
    │
    └──▶ Kairos ECL API (if enabled)
           │
           │  POST /api/v1/evaluate
           │  {
           │    signal_ids: [...],
           │    aggregated_risk: 0.87,
           │    context: { ... },
           │    trace_id: "trc-..."
           │  }
           │
           │  Response:
           │  {
           │    decision: "ESCALATE",
           │    confidence: 0.91,
           │    reasoning: "...",
           │    recommended_action: "notify_human"
           │  }
           │
           ▼
    Argus stores decision as a signal (Layer: L_DECISION)
    Dashboard shows decision in trace view
    Decision is auditable, queryable, correlatable
```

### 8.2 What Argus Provides to Kairos

Argus is the signal acquisition layer. Kairos receives:
- Aggregated risk scores per trace/conversation/session
- Signal timeline (ordered sequence of observations)
- Detection results (which rules fired, with what confidence)
- Context fields from relevant layers

Kairos returns a decision. Argus logs it. The operator sees both the signals that led to the decision and the decision itself. Full traceability.

### 8.3 What Kairos Is Not

Kairos is not a detection engine. It does not define what is anomalous. Argus does that. Kairos defines what to do about it. This separation is critical:
- Argus answers: "What happened? Is it anomalous?"
- Kairos answers: "Given what happened, what should the system do?"

---

## Part 9 — Configurability for End Users

### 9.1 Configuration Hierarchy

```
Global defaults (shipped with Argus)
  └── Organization overrides
        └── Environment overrides (prod vs staging)
              └── Application overrides (per app_id)
                    └── Rule-level overrides (per detection rule)
```

Every configuration parameter can be overridden at any level. Lower levels inherit from higher unless explicitly overridden. Changes are versioned and auditable.

### 9.2 What Operators Configure

**Signal ingestion**:
- Which layers to ingest (enable/disable per layer)
- Data redaction policies (hash PII fields, truncate prompts, strip embeddings)
- Sampling rates (ingest 100% of L6/L8 signals but 10% of L5 signals for cost control)
- Retention periods per layer and severity

**Detection**:
- Which rules are active
- Threshold tuning per rule (confidence, severity, baseline window)
- Custom rules (YAML upload + validation + dry run)
- Suppression windows (mute a rule for N hours during a known deployment)

**Alerting**:
- Routing rules (condition → channel mapping)
- Notification channels (Slack, PagerDuty, email, webhook, SIEM integration)
- Deduplication windows
- Escalation chains

**Access control**:
- RBAC: Admin (full), Analyst (read + investigate + tune), Viewer (read-only)
- API key scopes (per-app ingestion keys with write-only signal access)
- SSO integration (SAML, OIDC)

**Data governance**:
- Classification policies (which context fields are confidential)
- Retention policies (per-layer, per-severity, per-classification)
- Export restrictions (who can export raw signals)
- Audit log retention (immutable, configurable duration)

### 9.3 Configuration as Code

All configuration is expressible as YAML files that can be version-controlled. The `argus` CLI supports:

```bash
# Apply configuration from a file
argus config apply -f production-config.yaml

# Diff current config against a file
argus config diff -f proposed-changes.yaml

# Validate a detection rule
argus rules validate -f rules/new-rule.yaml

# Dry-run a rule against historical signals
argus rules test -f rules/new-rule.yaml --window 24h

# Export current config
argus config export -o current-config.yaml
```

This enables GitOps workflows: detection rules are reviewed in PRs, tested in CI, and deployed via `argus config apply`.

---

## Part 10 — Build Sequence

### Phase 0 — Schema & Protocol (Week 1–2)
- Define protobuf schema for ArgusSignal (the envelope + all per-layer context schemas)
- Define gRPC service definitions (Ingest, Query, Config, Health)
- Generate Go server stubs + Python/TS client stubs
- This is the contract. Everything else depends on it.

### Phase 1 — Ingest + Store (Week 3–5)
- Go ingestion server: gRPC + HTTP receivers
- Signal validation against protobuf schema
- ClickHouse schema design (table per layer vs single wide table — benchmark both)
- Write path: receive → validate → normalize → batch insert into ClickHouse
- Basic read path: query signals by time range, layer, app_id
- Health + metrics endpoints (Prometheus format)
- **Milestone**: SDK can send signals, they appear in ClickHouse, queryable via SQL

### Phase 2 — Processing Pipeline (Week 5–7)
- Enrichment: GeoIP, threat intel lookups, metadata augmentation
- Correlation: trace_id-based signal grouping, temporal window correlation
- Baseline engine: rolling statistical profiles per (app, layer, category)
- Pipeline is a Go channel-based DAG: receive → enrich → correlate → store → detect
- **Milestone**: Signals are enriched with baseline deviation scores

### Phase 3 — Detection Engine (Week 7–10)
- Rule parser: YAML → internal rule representation
- Tier 1 (deterministic): pattern match, threshold, schema validation
- Tier 2 (statistical): baseline deviation scoring (z-score, distribution shift)
- Tier 3 (temporal correlation): multi-signal pattern matching over windows
- Rule management API: CRUD, enable/disable, dry-run
- Alert generation: detection → alert record in PostgreSQL
- **Milestone**: Rules fire on incoming signals, alerts are generated

### Phase 4 — Alert & Response (Week 10–12)
- Alert routing engine: condition evaluation → channel dispatch
- Notification adapters: Slack, PagerDuty, email, webhook, generic HTTP
- Deduplication: content-based + time-window dedup
- Incident lifecycle: create, merge, escalate, resolve, close
- Incident correlation: group related alerts by trace/session/time
- **Milestone**: Detection → alert → Slack notification end-to-end

### Phase 5 — Dashboard (Week 12–16)
- React + TypeScript SPA scaffolding
- Coverage Map view (L1–L10 visual with real-time status)
- Signal Stream view (filterable, expandable, linked to trace)
- Trace View (full request lifecycle visualization)
- Detection Dashboard (per-rule performance metrics)
- Incident Timeline (multi-signal temporal visualization)
- Query Interface (SQL editor with autocomplete against ClickHouse schema)
- Configuration Editor (detection rules, alert routing, retention)
- WebSocket connection for live signal streaming
- **Milestone**: Operator can investigate an incident end-to-end via the UI

### Phase 6 — SDKs (Week 14–17, overlaps with Phase 5)
- Python SDK: decorator, context manager, manual, provider wrappers
- TypeScript SDK: middleware, manual
- OTEL bridge: OTLP receiver with Argus signal mapping
- SDK documentation + example applications
- **Milestone**: Three example apps (RAG, agent, chatbot) fully instrumented

### Phase 7 — Kairos Integration + Polish (Week 17–19)
- Kairos ECL gRPC client in Argus detection engine
- Decision signal type (L_DECISION) in ClickHouse schema
- Decision rendering in trace view
- Configuration for Kairos enable/disable, endpoint, policy
- End-to-end integration test: signal → detection → Kairos → decision → dashboard
- **Milestone**: Full Argus + Kairos stack operational

### Phase 8 — Production Hardening (Week 19–22)
- Load testing: 50K signals/sec sustained ingestion
- Chaos testing: ClickHouse node failure, SDK retry behavior, pipeline backpressure
- Security audit: API authentication, input validation, SQL injection in query interface
- RBAC implementation and testing
- Documentation: Architecture guide, operator manual, SDK reference, API reference
- Packaging: GitHub releases with checksums, container images on GHCR, Helm chart
- **Milestone**: Production-ready release

---

## Part 11 — What This Platform Is Not

Clarity on scope prevents scope creep and misaligned expectations.

**Argus is not a WAF.** It does not sit in the request path and block traffic. It observes, detects, and alerts. Blocking is the application's job (optionally informed by Kairos decisions).

**Argus is not a model training tool.** It does not fine-tune models, run RLHF, or manage training pipelines. It observes deployed models in production.

**Argus is not a prompt management platform.** It does not version prompts, A/B test system instructions, or manage prompt templates. It observes what prompts were sent and what happened.

**Argus is not a competitor to DataDog or Grafana for general APM.** It is purpose-built for the LLM signal surface. It complements general observability tools, does not replace them.

**Argus IS**: A specialized XDR platform that gives operators complete visibility into what their LLM-integrated systems are doing — across every layer of the stack, regardless of provider — with correlated detection, investigation workflows, and auditable decision trails.

---

## Appendix A — Signal Category Registry

Canonical category strings used in the `category` field. Extensible by operators.

```
# L1 — Hardware/Infrastructure
infra.gpu.utilization
infra.gpu.memory
infra.container.lifecycle
infra.network.throughput

# L2 — Model Weights
model.version.change
model.weight.integrity
model.quantization.event

# L3 — Tokenizer
tokenizer.encoding
tokenizer.anomaly
tokenizer.version_mismatch

# L4 — Transformer/Inference
inference.attention.distribution
inference.activation.magnitude
inference.kv_cache.utilization
inference.latency.per_layer

# L5 — Output/Decoding
output.generation
output.logprobs
output.truncation
output.streaming

# L6 — Safety/Alignment
safety.input_classifier
safety.output_filter
safety.policy_violation
safety.bypass_attempt

# L7 — RAG/Retrieval
retrieval.embedding
retrieval.search
retrieval.rerank
retrieval.chunk_inject
retrieval.index_health
retrieval.ingestion

# L8 — Agents/Tools
agent.tool_call
agent.tool_result
agent.planning
agent.delegation
agent.loop_iteration
agent.permission_request

# L9 — API Gateway
gateway.auth
gateway.rate_limit
gateway.routing
gateway.billing
gateway.error

# L10 — Application
app.session.start
app.session.end
app.user.message
app.user.feedback
app.conversation.metadata
app.error
```

## Appendix B — MITRE ATLAS Mapping

Every detection rule should reference the MITRE ATLAS tactic/technique it addresses. This makes Argus outputs legible to enterprise security teams who think in MITRE frameworks.

| ATLAS Technique | Argus Layer | Example Detection |
|----------------|-------------|-------------------|
| AML.T0015 — Evade ML Model | L6 | Jailbreak pattern detection |
| AML.T0016 — Obtain Capabilities | L9 | API key abuse / model extraction probing |
| AML.T0017 — Develop Capabilities | L8 | Agent privilege escalation |
| AML.T0020 — Poison Training Data | L7 | RAG chunk poisoning |
| AML.T0024 — Exfiltration via ML API | L8, L5 | PII in tool call / output |
| AML.T0040 — Prompt Injection | L3, L6, L7 | Direct and indirect injection |
| AML.T0043 — Craft Adversarial Data | L3 | Token-level adversarial inputs |
| AML.T0047 — Evade ML Model | L6 | Safety classifier evasion |

## Appendix C — What "Production-Grade" Means

A checklist. If any item is "no," you're not production-grade.

- [ ] Can a new operator install and see first signals in under 30 minutes?
- [ ] Can the platform sustain 10K signals/sec with sub-100ms detection latency?
- [ ] Can an operator investigate an incident from alert to root cause in the UI without touching SQL?
- [ ] Can detection rules be created, tested, and deployed without platform code changes?
- [ ] Can the platform tell the operator what it CAN'T see (coverage gaps)?
- [ ] Is every detection decision auditable with full signal lineage?
- [ ] Can the platform run without Docker? Without Kubernetes? On a single machine?
- [ ] Does the SDK add less than 5ms p99 overhead to instrumented applications?
- [ ] Can the operator control data retention, redaction, and classification per-layer?
- [ ] Is there a documented threat model that maps to the detection rule library?
- [ ] Can the platform integrate with existing SIEM/SOAR via standard protocols (syslog, CEF, webhook)?
- [ ] Is the configuration version-controlled and reviewable?
