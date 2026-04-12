package storage

// SignalsTableDDL defines the ClickHouse signals table schema.
// This table uses ReplacingMergeTree engine for deduplication support.
// All Layer context columns are Nullable since only one layer context is populated per signal.
const SignalsTableDDL = `
CREATE TABLE IF NOT EXISTS signals (
    -- Identity (from ArgusSignal)
    signal_id    String,
    trace_id     String,
    span_id      String,
    parent_span_id Nullable(String),

    -- Source (from Source message)
    app_id       String,
    app_version  String,
    host_id      Nullable(String),
    environment  Nullable(String),
    sdk_version  Nullable(String),

    -- Classification
    layer        UInt8,     -- Layer enum: L1=1 ... L10=10
    category     String,    -- e.g. "retrieval.search"
    severity     UInt8,     -- Severity enum: INFO=1...CRITICAL=5

    -- Temporal
    timestamp    DateTime64(9) CODEC(Delta, ZSTD),  -- nanosecond precision
    duration_ms  Nullable(Float32),
    ingested_at  DateTime64(3),

    -- Layer context (Nullable; only one is non-null per signal)
    -- L1 Hardware
    ctx_l1_cpu_percent          Nullable(Float32),
    ctx_l1_memory_used_mb       Nullable(Float32),
    ctx_l1_gpu_utilization_pct  Nullable(Float32),

    -- L2 Model Weights
    ctx_l2_model_id             Nullable(String),
    ctx_l2_model_hash           Nullable(String),
    ctx_l2_quantization         Nullable(String),

    -- L3 Tokenizer
    ctx_l3_input_token_count    Nullable(UInt32),
    ctx_l3_output_token_count   Nullable(UInt32),
    ctx_l3_truncated            Nullable(UInt8),

    -- L4 Transformer
    ctx_l4_attention_entropy    Nullable(Float32),
    ctx_l4_kv_cache_hit_rate    Nullable(Float32),

    -- L5 Output/Decoding
    ctx_l5_mean_logprob         Nullable(Float32),
    ctx_l5_top_logprob          Nullable(Float32),
    ctx_l5_finish_reason        Nullable(String),

    -- L6 Safety
    ctx_l6_safety_score         Nullable(Float32),
    ctx_l6_policy_violated      Nullable(String),
    ctx_l6_action_taken         Nullable(String),

    -- L7 RAG/Retrieval
    ctx_l7_query_text           Nullable(String),
    ctx_l7_retrieved_count      Nullable(UInt32),
    ctx_l7_top_score            Nullable(Float32),
    ctx_l7_collection_name      Nullable(String),

    -- L8 Agents
    ctx_l8_tool_name            Nullable(String),
    ctx_l8_tool_input_hash      Nullable(String),
    ctx_l8_agent_step           Nullable(UInt32),

    -- L9 API Gateway
    ctx_l9_method               Nullable(String),
    ctx_l9_path                 Nullable(String),
    ctx_l9_status_code          Nullable(UInt16),
    ctx_l9_latency_ms           Nullable(Float32),

    -- L10 Application
    ctx_l10_event_type          Nullable(String),
    ctx_l10_component           Nullable(String),

    -- Relationships
    related_signals  Array(String),
    incident_id      Nullable(String),
    session_id       Nullable(String),
    conversation_id  Nullable(String),
    user_id          Nullable(String),

    -- Provider
    provider_name    Nullable(String),
    provider_model   Nullable(String),

    -- Enrichment (populated by pipeline, Phase 3+)
    enrich_baseline_deviation   Nullable(Float32),
    enrich_geoip_country        Nullable(String),
    enrich_geoip_city           Nullable(String),
    enrich_threat_intel_hit     Nullable(UInt8),

    -- Governance
    data_classification  UInt8,   -- DataClassification enum
    retention_policy     String,
    pii_detected         UInt8,   -- bool as UInt8

    -- Internal (deduplication version)
    version  UInt32 DEFAULT 1
) ENGINE = ReplacingMergeTree(version)
  ORDER BY (app_id, layer, timestamp)
  PARTITION BY toYYYYMM(timestamp)
  TTL CAST(timestamp, 'DateTime') + INTERVAL 90 DAY
  SETTINGS index_granularity = 8192
`

// ArgusSignalFieldMapping documents the relationship between ArgusSignal proto fields
// and ClickHouse signal table columns. This is used for validation and documentation.
type ArgusSignalFieldMapping struct {
	// Identity fields (required)
	SignalID     string // signal_id
	TraceID      string // trace_id
	SpanID       string // span_id
	ParentSpanID string // parent_span_id (optional)

	// Source fields (required)
	AppID      string // app_id
	AppVersion string // app_version
	HostID     string // host_id (from source metadata)
	Environment string // environment
	SDKVersion string // sdk_version

	// Classification fields (required)
	Layer    uint8  // layer (0-10)
	Category string // category
	Severity uint8  // severity (0-5)

	// Temporal fields (required)
	Timestamp   int64   // timestamp (unix nanos)
	DurationMS  float32 // duration_ms (optional)
	IngestedAt  int64   // ingested_at (unix millis)

	// Relationships (optional)
	RelatedSignals []string
	IncidentID     string
	SessionID      string
	ConversationID string
	UserID         string

	// Provider (optional)
	ProviderName  string
	ProviderModel string

	// Governance (required)
	DataClassification uint8  // classification enum
	RetentionPolicy    string // retention policy name
	PiiDetected        uint8  // bool as uint8
}
