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
    ctx_l1_cpu_usage_pct            Nullable(Float32),
    ctx_l1_memory_used_mb           Nullable(Float32),
    ctx_l1_memory_total_mb          Nullable(Float32),
    ctx_l1_gpu_utilization_pct      Nullable(Float32),
    ctx_l1_gpu_memory_used_mb       Nullable(Float32),
    ctx_l1_gpu_memory_total_mb      Nullable(Float32),
    ctx_l1_gpu_temperature_c        Nullable(Float32),
    ctx_l1_gpu_power_draw_w         Nullable(Float32),
    ctx_l1_gpu_count                Nullable(Int32),
    ctx_l1_gpu_model                Nullable(String),
    ctx_l1_disk_read_mbps           Nullable(Float32),
    ctx_l1_disk_write_mbps          Nullable(Float32),
    ctx_l1_net_rx_mbps              Nullable(Float32),
    ctx_l1_net_tx_mbps              Nullable(Float32),
    ctx_l1_cpu_steal_pct            Nullable(Float32),
    ctx_l1_load_avg_1m              Nullable(Float32),
    ctx_l1_load_avg_5m              Nullable(Float32),
    ctx_l1_load_avg_15m             Nullable(Float32),
    ctx_l1_open_file_descriptors    Nullable(Int32),
    ctx_l1_thread_count             Nullable(Int32),
    ctx_l1_host_id                  Nullable(String),
    ctx_l1_instance_type            Nullable(String),
    ctx_l1_process_cpu_pct          Nullable(Float32),
    ctx_l1_process_memory_mb        Nullable(Float32),

    -- L2 Model Weights
    ctx_l2_model_id                 Nullable(String),
    ctx_l2_model_family             Nullable(String),
    ctx_l2_model_version            Nullable(String),
    ctx_l2_model_hash               Nullable(String),
    ctx_l2_quantization             Nullable(String),
    ctx_l2_parameter_count          Nullable(Int64),
    ctx_l2_model_size_gb            Nullable(Float32),
    ctx_l2_load_time_ms             Nullable(Float32),
    ctx_l2_vram_allocated_gb        Nullable(Float32),
    ctx_l2_ram_allocated_gb         Nullable(Float32),
    ctx_l2_precision                Nullable(String),
    ctx_l2_backend                  Nullable(String),
    ctx_l2_adapter_id               Nullable(String),
    ctx_l2_adapter_hash             Nullable(String),
    ctx_l2_weights_verified         Nullable(UInt8),
    ctx_l2_weights_load_latency_ms  Nullable(Float32),
    ctx_l2_layers_offloaded         Nullable(Int32),
    ctx_l2_device_map               Nullable(String),

    -- L3 Tokenizer
    ctx_l3_input_tokens             Nullable(Int32),
    ctx_l3_output_tokens            Nullable(Int32),
    ctx_l3_total_tokens             Nullable(Int32),
    ctx_l3_tokenizer_id             Nullable(String),
    ctx_l3_tokenizer_version        Nullable(String),
    ctx_l3_encoding_time_ms         Nullable(Float32),
    ctx_l3_decoding_time_ms         Nullable(Float32),
    ctx_l3_context_window_size      Nullable(Int32),
    ctx_l3_context_utilization_pct  Nullable(Float32),
    ctx_l3_truncated_tokens         Nullable(Int32),
    ctx_l3_truncated                Nullable(UInt8),
    ctx_l3_chat_template            Nullable(String),
    ctx_l3_bos_token                Nullable(String),
    ctx_l3_avg_token_length         Nullable(Float32),
    ctx_l3_unique_token_count       Nullable(Int32),
    ctx_l3_token_entropy            Nullable(Float32),
    ctx_l3_special_token_count      Nullable(Int32),
    ctx_l3_encoding_name            Nullable(String),
    ctx_l3_vocab_size               Nullable(Int32),
    ctx_l3_compression_ratio        Nullable(Float32),
    ctx_l3_oov_rate                 Nullable(Float32),
    ctx_l3_has_special_tokens       Nullable(UInt8),

    -- L4 Transformer
    ctx_l4_kv_cache_used_mb         Nullable(Float32),
    ctx_l4_kv_cache_total_mb        Nullable(Float32),
    ctx_l4_kv_cache_hit_rate        Nullable(Float32),
    ctx_l4_num_heads                Nullable(Int32),
    ctx_l4_num_layers               Nullable(Int32),
    ctx_l4_attention_entropy_mean   Nullable(Float32),
    ctx_l4_attention_entropy_max    Nullable(Float32),
    ctx_l4_attention_entropy_min    Nullable(Float32),
    ctx_l4_prefill_time_ms          Nullable(Float32),
    ctx_l4_decode_time_ms           Nullable(Float32),
    ctx_l4_batch_size               Nullable(Int32),
    ctx_l4_sequence_length          Nullable(Int32),
    ctx_l4_throughput_tokens_per_sec Nullable(Float32),
    ctx_l4_memory_bandwidth_gbps    Nullable(Float32),
    ctx_l4_flash_attention          Nullable(UInt8),
    ctx_l4_paged_attention          Nullable(UInt8),
    ctx_l4_speculative_tokens       Nullable(Int32),
    ctx_l4_speculation_accept_rate  Nullable(Float32),
    ctx_l4_context_length_used      Nullable(Int32),
    ctx_l4_active_sequences         Nullable(Int32),
    ctx_l4_flops_utilized           Nullable(Float32),
    ctx_l4_attention_implementation Nullable(String),
    ctx_l4_compute_dtype            Nullable(String),

    -- L5 Output/Decoding
    ctx_l5_operation                Nullable(Int32),
    ctx_l5_output_tokens            Nullable(Int32),
    ctx_l5_input_tokens             Nullable(Int32),
    ctx_l5_total_tokens             Nullable(Int32),
    ctx_l5_finish_reason            Nullable(String),
    ctx_l5_temperature              Nullable(Float32),
    ctx_l5_top_p                    Nullable(Float32),
    ctx_l5_mean_logprob             Nullable(Float32),
    ctx_l5_min_logprob              Nullable(Float32),
    ctx_l5_entropy_mean             Nullable(Float32),
    ctx_l5_entropy_variance         Nullable(Float32),
    ctx_l5_ttft_ms                  Nullable(Float32),
    ctx_l5_tps                      Nullable(Float32),
    ctx_l5_top_k                    Nullable(Int32),
    ctx_l5_min_p                    Nullable(Float32),
    ctx_l5_repetition_penalty       Nullable(Float32),
    ctx_l5_presence_penalty         Nullable(Float32),
    ctx_l5_frequency_penalty        Nullable(Float32),
    ctx_l5_seed                     Nullable(Int64),
    ctx_l5_stop_sequences           Array(String),
    ctx_l5_output_repetition_score  Nullable(Float32),
    ctx_l5_distinct_ngrams_ratio    Nullable(Float32),
    ctx_l5_truncated                Nullable(UInt8),
    ctx_l5_finish_reason_detail     Nullable(String),

    -- L6 Safety
    ctx_l6_safety_score             Nullable(Float32),
    ctx_l6_safe                     Nullable(UInt8),
    ctx_l6_action_taken             Nullable(String),
    ctx_l6_classifier_id            Nullable(String),
    ctx_l6_category_scores          Nullable(String),   -- JSON array of SafetyCategoryScore
    ctx_l6_toxicity_score           Nullable(Float32),
    ctx_l6_bias_score               Nullable(Float32),
    ctx_l6_policy_id                Nullable(String),
    ctx_l6_policy_version           Nullable(String),
    ctx_l6_policy_violated          Nullable(UInt8),
    ctx_l6_violation_type           Nullable(String),
    ctx_l6_redaction_method         Nullable(String),
    ctx_l6_redacted_tokens          Nullable(Int32),
    ctx_l6_evaluation_time_ms       Nullable(Float32),
    ctx_l6_input_language           Nullable(String),
    ctx_l6_language_confidence      Nullable(Float32),
    ctx_l6_jailbreak_detected       Nullable(UInt8),
    ctx_l6_jailbreak_score          Nullable(Float32),
    ctx_l6_prompt_injection_detected Nullable(UInt8),
    ctx_l6_prompt_injection_score   Nullable(Float32),
    ctx_l6_pii_types_detected       Nullable(String),
    ctx_l6_pii_redacted             Nullable(UInt8),

    -- L7 RAG/Retrieval
    ctx_l7_operation                Nullable(Int32),
    ctx_l7_query_text               Nullable(String),
    ctx_l7_results_count            Nullable(Int32),
    ctx_l7_top_chunk_source         Nullable(String),
    ctx_l7_context_window_pct       Nullable(Float32),
    ctx_l7_embedding_model          Nullable(String),
    ctx_l7_vector_index             Nullable(String),
    ctx_l7_reranker_applied         Nullable(UInt8),
    ctx_l7_reranker_delta           Nullable(Float32),
    ctx_l7_latency_embedding_ms     Nullable(Float32),
    ctx_l7_latency_search_ms        Nullable(Float32),
    ctx_l7_latency_rerank_ms        Nullable(Float32),
    ctx_l7_query_cache_hit          Nullable(UInt8),
    ctx_l7_query_cache_key          Nullable(String),
    ctx_l7_citations_used           Nullable(Int32),
    ctx_l7_citation_overlap_score   Nullable(Float32),
    ctx_l7_index_name               Nullable(String),
    ctx_l7_index_type               Nullable(String),
    ctx_l7_index_document_count     Nullable(Int64),
    ctx_l7_index_freshness_hours    Nullable(Float32),
    ctx_l7_reranker_score           Nullable(Float32),
    ctx_l7_reranker_model           Nullable(String),
    ctx_l7_hybrid_search            Nullable(UInt8),
    ctx_l7_sparse_weight            Nullable(Float32),
    ctx_l7_dense_weight             Nullable(Float32),

    -- L8 Agents
    ctx_l8_operation                Nullable(Int32),
    ctx_l8_tool_name                Nullable(String),
    ctx_l8_tool_provider            Nullable(String),
    ctx_l8_tool_result              Nullable(String),
    ctx_l8_tool_error               Nullable(String),
    ctx_l8_tool_latency_ms          Nullable(Float32),
    ctx_l8_step_number              Nullable(Int32),
    ctx_l8_total_steps              Nullable(Int32),
    ctx_l8_permissions_used         Array(String),
    ctx_l8_permissions_requested    Array(String),
    ctx_l8_data_flow_tags           Array(String),
    ctx_l8_task_id                  Nullable(String),
    ctx_l8_parent_task_id           Nullable(String),
    ctx_l8_task_depth               Nullable(Int32),
    ctx_l8_subtask_count            Nullable(Int32),
    ctx_l8_memory_operation         Nullable(String),
    ctx_l8_memory_items_read        Nullable(Int32),
    ctx_l8_memory_items_written     Nullable(Int32),
    ctx_l8_code_language            Nullable(String),
    ctx_l8_code_executed            Nullable(UInt8),
    ctx_l8_code_exit_code           Nullable(Int32),
    ctx_l8_code_execution_time_ms   Nullable(Float32),
    ctx_l8_sandboxed                Nullable(UInt8),
    ctx_l8_capabilities_used        Array(String),
    ctx_l8_orchestrator_type        Nullable(String),

    -- L9 API Gateway
    ctx_l9_method                   Nullable(String),
    ctx_l9_path                     Nullable(String),
    ctx_l9_status_code              Nullable(Int32),
    ctx_l9_latency_ms               Nullable(Float32),
    ctx_l9_client_ip                Nullable(String),
    ctx_l9_user_agent               Nullable(String),
    ctx_l9_api_key_id               Nullable(String),
    ctx_l9_api_version              Nullable(String),
    ctx_l9_request_size_bytes       Nullable(Int64),
    ctx_l9_response_size_bytes      Nullable(Int64),
    ctx_l9_cache_hit                Nullable(UInt8),
    ctx_l9_cache_key                Nullable(String),
    ctx_l9_rate_limit_remaining     Nullable(Float32),
    ctx_l9_rate_limit_reset_ms      Nullable(Float32),
    ctx_l9_rate_limited             Nullable(UInt8),
    ctx_l9_auth_method              Nullable(String),
    ctx_l9_authenticated            Nullable(UInt8),
    ctx_l9_client_id                Nullable(String),
    ctx_l9_request_id               Nullable(String),
    ctx_l9_upstream_id              Nullable(String),
    ctx_l9_upstream_latency_ms      Nullable(Float32),
    ctx_l9_ssl_terminated           Nullable(UInt8),
    ctx_l9_protocol                 Nullable(String),
    ctx_l9_region                   Nullable(String),
    ctx_l9_endpoint_alias           Nullable(String),

    -- L10 Application
    ctx_l10_user_id                 Nullable(String),
    ctx_l10_session_id              Nullable(String),
    ctx_l10_conversation_id         Nullable(String),
    ctx_l10_event_type              Nullable(String),
    ctx_l10_component               Nullable(String),
    ctx_l10_action                  Nullable(String),
    ctx_l10_feature_id              Nullable(String),
    ctx_l10_ab_variant              Nullable(String),
    ctx_l10_request_retry_count     Nullable(Int32),
    ctx_l10_client_latency_ms       Nullable(Float32),
    ctx_l10_error_code              Nullable(String),
    ctx_l10_error_message           Nullable(String),
    ctx_l10_error_recovered         Nullable(UInt8),
    ctx_l10_platform                Nullable(String),
    ctx_l10_app_version             Nullable(String),
    ctx_l10_locale                  Nullable(String),
    ctx_l10_timezone                Nullable(String),
    ctx_l10_feature_flags           Map(String, String),
    ctx_l10_custom_attributes       Map(String, String),
    ctx_l10_page_id                 Nullable(String),
    ctx_l10_referrer                Nullable(String),
    ctx_l10_satisfaction_score      Nullable(Float32),
    ctx_l10_feedback_provided       Nullable(UInt8),
    ctx_l10_deployment_id           Nullable(String),

    -- LDecision (Kairos policy decisions)
    ctx_ld_decision                 Nullable(Int32),
    ctx_ld_confidence               Nullable(Float32),
    ctx_ld_reasoning                Nullable(String),
    ctx_ld_recommended_action       Nullable(Int32),
    ctx_ld_policy_version           Nullable(String),
    ctx_ld_policy_name              Nullable(String),
    ctx_ld_evaluation_time_ms       Nullable(Float32),
    ctx_ld_policy_id                Nullable(String),
    ctx_ld_triggering_rule_id       Nullable(String),
    ctx_ld_triggering_signal_id     Nullable(String),
    ctx_ld_failed_open              Nullable(UInt8),
    ctx_ld_decision_trace_id        Nullable(String),
    ctx_ld_alert_threshold          Nullable(Float32),
    ctx_ld_matched_indicators       Array(String),

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
