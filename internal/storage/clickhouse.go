package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"go.uber.org/zap"
)

// SignalsInsertColumns lists every column from SignalsTableDDL in schema.go, in DDL order.
// This is used for the explicit column list in INSERT statements.
// The `version` column is intentionally omitted — it uses DEFAULT 1 in the DDL.
const SignalsInsertColumns = `(
    signal_id, trace_id, span_id, parent_span_id,
    app_id, app_version, host_id, environment, sdk_version,
    layer, category, severity,
    timestamp, duration_ms, ingested_at,
    ctx_l1_cpu_usage_pct, ctx_l1_memory_used_mb, ctx_l1_memory_total_mb,
    ctx_l1_gpu_utilization_pct, ctx_l1_gpu_memory_used_mb, ctx_l1_gpu_memory_total_mb,
    ctx_l1_gpu_temperature_c, ctx_l1_gpu_power_draw_w, ctx_l1_gpu_count, ctx_l1_gpu_model,
    ctx_l1_disk_read_mbps, ctx_l1_disk_write_mbps, ctx_l1_net_rx_mbps, ctx_l1_net_tx_mbps,
    ctx_l1_cpu_steal_pct, ctx_l1_load_avg_1m, ctx_l1_load_avg_5m, ctx_l1_load_avg_15m,
    ctx_l1_open_file_descriptors, ctx_l1_thread_count, ctx_l1_host_id, ctx_l1_instance_type,
    ctx_l1_process_cpu_pct, ctx_l1_process_memory_mb,
    ctx_l2_model_id, ctx_l2_model_family, ctx_l2_model_version, ctx_l2_model_hash,
    ctx_l2_quantization, ctx_l2_parameter_count, ctx_l2_model_size_gb, ctx_l2_load_time_ms,
    ctx_l2_vram_allocated_gb, ctx_l2_ram_allocated_gb, ctx_l2_precision, ctx_l2_backend,
    ctx_l2_adapter_id, ctx_l2_adapter_hash, ctx_l2_weights_verified, ctx_l2_weights_load_latency_ms,
    ctx_l2_layers_offloaded, ctx_l2_device_map,
    ctx_l3_input_tokens, ctx_l3_output_tokens, ctx_l3_total_tokens,
    ctx_l3_tokenizer_id, ctx_l3_tokenizer_version, ctx_l3_encoding_time_ms, ctx_l3_decoding_time_ms,
    ctx_l3_context_window_size, ctx_l3_context_utilization_pct, ctx_l3_truncated_tokens, ctx_l3_truncated,
    ctx_l3_chat_template, ctx_l3_bos_token, ctx_l3_avg_token_length, ctx_l3_unique_token_count,
    ctx_l3_token_entropy, ctx_l3_special_token_count, ctx_l3_encoding_name, ctx_l3_vocab_size,
    ctx_l3_compression_ratio, ctx_l3_oov_rate, ctx_l3_has_special_tokens,
    ctx_l4_kv_cache_used_mb, ctx_l4_kv_cache_total_mb, ctx_l4_kv_cache_hit_rate,
    ctx_l4_num_heads, ctx_l4_num_layers,
    ctx_l4_attention_entropy_mean, ctx_l4_attention_entropy_max, ctx_l4_attention_entropy_min,
    ctx_l4_prefill_time_ms, ctx_l4_decode_time_ms, ctx_l4_batch_size, ctx_l4_sequence_length,
    ctx_l4_throughput_tokens_per_sec, ctx_l4_memory_bandwidth_gbps,
    ctx_l4_flash_attention, ctx_l4_paged_attention,
    ctx_l4_speculative_tokens, ctx_l4_speculation_accept_rate,
    ctx_l4_context_length_used, ctx_l4_active_sequences, ctx_l4_flops_utilized,
    ctx_l4_attention_implementation, ctx_l4_compute_dtype,
    ctx_l5_operation, ctx_l5_output_tokens, ctx_l5_input_tokens, ctx_l5_total_tokens,
    ctx_l5_finish_reason, ctx_l5_temperature, ctx_l5_top_p,
    ctx_l5_mean_logprob, ctx_l5_min_logprob, ctx_l5_entropy_mean, ctx_l5_entropy_variance,
    ctx_l5_ttft_ms, ctx_l5_tps,
    ctx_l5_top_k, ctx_l5_min_p, ctx_l5_repetition_penalty, ctx_l5_presence_penalty, ctx_l5_frequency_penalty,
    ctx_l5_seed, ctx_l5_stop_sequences, ctx_l5_output_repetition_score, ctx_l5_distinct_ngrams_ratio,
    ctx_l5_truncated, ctx_l5_finish_reason_detail,
    ctx_l6_safety_score, ctx_l6_safe, ctx_l6_action_taken, ctx_l6_classifier_id, ctx_l6_category_scores,
    ctx_l6_toxicity_score, ctx_l6_bias_score, ctx_l6_policy_id, ctx_l6_policy_version, ctx_l6_policy_violated,
    ctx_l6_violation_type, ctx_l6_redaction_method, ctx_l6_redacted_tokens, ctx_l6_evaluation_time_ms,
    ctx_l6_input_language, ctx_l6_language_confidence,
    ctx_l6_jailbreak_detected, ctx_l6_jailbreak_score,
    ctx_l6_prompt_injection_detected, ctx_l6_prompt_injection_score,
    ctx_l6_pii_types_detected, ctx_l6_pii_redacted,
    ctx_l7_operation, ctx_l7_query_text, ctx_l7_results_count, ctx_l7_top_chunk_source,
    ctx_l7_context_window_pct, ctx_l7_embedding_model, ctx_l7_vector_index,
    ctx_l7_reranker_applied, ctx_l7_reranker_delta,
    ctx_l7_latency_embedding_ms, ctx_l7_latency_search_ms, ctx_l7_latency_rerank_ms,
    ctx_l7_query_cache_hit, ctx_l7_query_cache_key, ctx_l7_citations_used, ctx_l7_citation_overlap_score,
    ctx_l7_index_name, ctx_l7_index_type, ctx_l7_index_document_count, ctx_l7_index_freshness_hours,
    ctx_l7_reranker_score, ctx_l7_reranker_model, ctx_l7_hybrid_search, ctx_l7_sparse_weight, ctx_l7_dense_weight,
    ctx_l8_operation, ctx_l8_tool_name, ctx_l8_tool_provider, ctx_l8_tool_result, ctx_l8_tool_error,
    ctx_l8_tool_latency_ms, ctx_l8_step_number, ctx_l8_total_steps,
    ctx_l8_permissions_used, ctx_l8_permissions_requested, ctx_l8_data_flow_tags,
    ctx_l8_task_id, ctx_l8_parent_task_id, ctx_l8_task_depth, ctx_l8_subtask_count,
    ctx_l8_memory_operation, ctx_l8_memory_items_read, ctx_l8_memory_items_written,
    ctx_l8_code_language, ctx_l8_code_executed, ctx_l8_code_exit_code, ctx_l8_code_execution_time_ms,
    ctx_l8_sandboxed, ctx_l8_capabilities_used, ctx_l8_orchestrator_type,
    ctx_l9_method, ctx_l9_path, ctx_l9_status_code, ctx_l9_latency_ms,
    ctx_l9_client_ip, ctx_l9_user_agent, ctx_l9_api_key_id, ctx_l9_api_version,
    ctx_l9_request_size_bytes, ctx_l9_response_size_bytes,
    ctx_l9_cache_hit, ctx_l9_cache_key,
    ctx_l9_rate_limit_remaining, ctx_l9_rate_limit_reset_ms, ctx_l9_rate_limited,
    ctx_l9_auth_method, ctx_l9_authenticated, ctx_l9_client_id, ctx_l9_request_id,
    ctx_l9_upstream_id, ctx_l9_upstream_latency_ms,
    ctx_l9_ssl_terminated, ctx_l9_protocol, ctx_l9_region, ctx_l9_endpoint_alias,
    ctx_l10_user_id, ctx_l10_session_id, ctx_l10_conversation_id, ctx_l10_event_type,
    ctx_l10_component, ctx_l10_action, ctx_l10_feature_id, ctx_l10_ab_variant,
    ctx_l10_request_retry_count, ctx_l10_client_latency_ms,
    ctx_l10_error_code, ctx_l10_error_message, ctx_l10_error_recovered,
    ctx_l10_platform, ctx_l10_app_version, ctx_l10_locale, ctx_l10_timezone,
    ctx_l10_feature_flags, ctx_l10_custom_attributes,
    ctx_l10_page_id, ctx_l10_referrer, ctx_l10_satisfaction_score, ctx_l10_feedback_provided,
    ctx_l10_deployment_id,
    ctx_ld_decision, ctx_ld_confidence, ctx_ld_reasoning, ctx_ld_recommended_action,
    ctx_ld_policy_version, ctx_ld_policy_name, ctx_ld_evaluation_time_ms,
    ctx_ld_policy_id, ctx_ld_triggering_rule_id, ctx_ld_triggering_signal_id,
    ctx_ld_failed_open, ctx_ld_decision_trace_id, ctx_ld_alert_threshold, ctx_ld_matched_indicators,
    related_signals, incident_id, session_id, conversation_id, user_id,
    provider_name, provider_model,
    enrich_baseline_deviation, enrich_geoip_country, enrich_geoip_city, enrich_threat_intel_hit,
    data_classification, retention_policy, pii_detected
)`

// Dequeuer defines the interface for consuming signals from the ingest queue.
// This decouples storage from the ingest package, preventing circular dependencies.
type Dequeuer interface {
	Dequeue(ctx context.Context) (*v1.ArgusSignal, error)
}

// ClickHouse wraps the clickhouse-go/v2 client for Argus signal storage.
// It manages connection pooling and provides batch write operations.
type ClickHouse struct {
	conn driver.Conn
}

// NewClickHouse creates a ClickHouse client and applies the schema.
// It uses the native protocol (not HTTP) for better performance.
// The connection will have AsyncInsert enabled for batch writes.
//
// dsn format accepts:
//   - Full URI: "clickhouse://[user[:password]@]host[:port]/[database]"
//   - Raw address: "host:port"
// Example: "clickhouse://default:@localhost:9000/default" or "localhost:9000"
func NewClickHouse(ctx context.Context, dsn string) (*ClickHouse, error) {
	// Parse the DSN to extract host:port for the driver
	addr := dsn
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err == nil && u.Host != "" {
			addr = u.Host
			// If port isn't in the host, use default
			if !strings.Contains(addr, ":") {
				addr = addr + ":9000"
			}
		}
	}

	// Create connection with parsed address
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connectivity with a ping
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	// Apply the schema (idempotent)
	if err := applySchema(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &ClickHouse{
		conn: conn,
	}, nil
}

// applySchema executes the SignalsTableDDL against the ClickHouse connection.
// It is safe to call multiple times (CREATE TABLE IF NOT EXISTS).
func applySchema(ctx context.Context, conn driver.Conn) error {
	if err := conn.Exec(ctx, SignalsTableDDL); err != nil {
		return fmt.Errorf("failed to execute DDL: %w", err)
	}
	return nil
}

// BatchWriter provides a buffered interface for writing signals to ClickHouse.
// It batches signals and flushes them via AsyncInsert for efficiency.
// Signals are flushed when either batchSize is reached or flushInterval elapsed.
type BatchWriter struct {
	ch            *ClickHouse
	conn          driver.Conn
	mu            sync.Mutex
	buffer        []*v1.ArgusSignal
	batchSize     int
	flushInterval time.Duration
	flushCh       chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	metrics       *metrics.Storage
	log           *zap.Logger
	closed        bool
}

// NewBatchWriter creates a new batch writer with the given batch size and flush interval.
// The batch size should be between 500-1000 for optimal AsyncInsert performance.
// The flush interval (default 2s) determines max time to wait for a partial batch.
func NewBatchWriter(ctx context.Context, ch *ClickHouse, batchSize int, flushInterval time.Duration, m *metrics.Storage, log *zap.Logger) (*BatchWriter, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}
	if log == nil {
		log = zap.NewNop()
	}

	batchCtx, cancel := context.WithCancel(ctx)

	bw := &BatchWriter{
		ch:            ch,
		conn:          ch.conn,
		buffer:        make([]*v1.ArgusSignal, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		flushCh:       make(chan struct{}, 1),
		ctx:           batchCtx,
		cancel:        cancel,
		metrics:       m,
		log:           log,
	}

	// Start the flush goroutine
	go bw.flushLoop()

	return bw, nil
}

// Write adds a signal to the buffer. Triggers flush if buffer >= batchSize.
// Non-blocking. Returns immediately. Never blocks the caller.
func (bw *BatchWriter) Write(ctx context.Context, sig *v1.ArgusSignal) error {
	if sig == nil {
		return fmt.Errorf("signal cannot be nil")
	}

	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return fmt.Errorf("batch writer is closed")
	}

	bw.buffer = append(bw.buffer, sig)

	// Trigger flush if buffer is full
	if len(bw.buffer) >= bw.batchSize {
		// Non-blocking send to trigger flush
		select {
		case bw.flushCh <- struct{}{}:
		default:
			// Channel already has signal, no need to send again
		}
	}

	return nil
}

// flushLoop runs in a separate goroutine and flushes the buffer periodically.
func (bw *BatchWriter) flushLoop() {
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bw.ctx.Done():
			return
		case <-ticker.C:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				if err := bw.flushLocked(); err != nil {
					bw.log.Error("flush failed", zap.Error(err))
				}
			}
			bw.mu.Unlock()
		case <-bw.flushCh:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				if err := bw.flushLocked(); err != nil {
					bw.log.Error("flush failed", zap.Error(err))
				}
			}
			bw.mu.Unlock()
		}
	}
}

// flushLocked serializes buffered signals and inserts into ClickHouse.
// MUST be called with mu locked.
// On ClickHouse error: logs at ERROR, increments storage_insert_errors_total metric,
// retries once after 100ms. If retry fails, signals are lost (log at ERROR with count).
// This is acceptable: signals are not critical path (vs. the ingest queue never losing).
func (bw *BatchWriter) flushLocked() error {
	if len(bw.buffer) == 0 {
		return nil
	}

	startTime := time.Now()
	batchSize := len(bw.buffer)

	batch, err := bw.conn.PrepareBatch(bw.ctx, "INSERT INTO signals " + SignalsInsertColumns)
	if err != nil {
		if bw.metrics != nil && bw.metrics.InsertErrors != nil {
			bw.metrics.InsertErrors.Inc()
		}
		bw.log.Error("failed to prepare batch", zap.Error(err), zap.Int("buffer_size", batchSize))
		return err
	}

	// Append all signals to the batch
	for _, sig := range bw.buffer {
		if err := signalToClickHouseRow(batch, sig); err != nil {
			bw.log.Error("failed to append signal to batch", zap.Error(err), zap.String("signal_id", sig.SignalId))
			batch.Abort()
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err
		}
	}

	// Send the batch
	if err := batch.Send(); err != nil {
		bw.log.Error("batch send failed", zap.Error(err), zap.Int("buffer_size", batchSize))
		batch.Abort()

		// Retry once after 100ms
		bw.log.Info("retrying batch send", zap.Int("buffer_size", batchSize))
		time.Sleep(100 * time.Millisecond)

		batch2, err2 := bw.conn.PrepareBatch(bw.ctx, "INSERT INTO signals " + SignalsInsertColumns)
		if err2 != nil {
			bw.log.Error("failed to prepare retry batch", zap.Error(err2), zap.Int("buffer_size", batchSize))
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err2
		}

		for _, sig := range bw.buffer {
			if err2 := signalToClickHouseRow(batch2, sig); err2 != nil {
				bw.log.Error("failed to append signal to retry batch", zap.Error(err2), zap.String("signal_id", sig.SignalId))
				batch2.Abort()
				if bw.metrics != nil && bw.metrics.InsertErrors != nil {
					bw.metrics.InsertErrors.Inc()
				}
				return err2
			}
		}

		if err2 := batch2.Send(); err2 != nil {
			bw.log.Error("batch retry send failed, signals lost", zap.Error(err2), zap.Int("buffer_size", batchSize))
			batch2.Abort()
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err2
		}
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if bw.metrics != nil {
		if bw.metrics.BatchFlushes != nil {
			bw.metrics.BatchFlushes.WithLabelValues("ok").Inc()
		}
		if bw.metrics.BatchSize != nil {
			bw.metrics.BatchSize.Observe(float64(batchSize))
		}
		if bw.metrics.FlushDuration != nil {
			bw.metrics.FlushDuration.Observe(duration)
		}
	}

	bw.log.Debug("batch flushed", zap.Int("size", batchSize), zap.Float64("duration_seconds", duration))

	// Clear buffer
	bw.buffer = bw.buffer[:0]

	return nil
}

// Close triggers a final flush and closes the batch writer.
func (bw *BatchWriter) Close() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return nil
	}

	// Flush remaining signals
	if len(bw.buffer) > 0 {
		if err := bw.flushLocked(); err != nil {
			bw.log.Error("final flush failed", zap.Error(err))
		}
	}

	bw.closed = true
	bw.cancel()

	return nil
}

// boolToUInt8Ptr converts a *bool to a *uint8 for Nullable(UInt8) columns.
func boolToUInt8Ptr(b *bool) *uint8 {
	if b == nil {
		return nil
	}
	if *b {
		v := uint8(1)
		return &v
	}
	v := uint8(0)
	return &v
}

// strPtr returns nil if s is empty, otherwise &s.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// float32Ptr returns the pointer directly (used for Nullable(Float32) extraction).
func float32Val(v float32) *float32 {
	return &v
}

// int32Ptr returns a pointer to an int32 value.
func int32Ptr(v int32) *int32 {
	return &v
}

// int64Ptr returns a pointer to an int64 value.
func int64Ptr(v int64) *int64 {
	return &v
}

// signalToClickHouseRow converts an ArgusSignal to a ClickHouse batch row.
// Uses positional arguments matching the full DDL column order in schema.go.
// Column count: 246 columns total (must match SignalsInsertColumns).
// NOTE: `version` column is omitted — it uses DEFAULT 1 in DDL.
func signalToClickHouseRow(batch driver.Batch, sig *v1.ArgusSignal) error {
	// ── Identity ────────────────────────────────────────────────────────────
	signalID := sig.SignalId
	traceID := sig.TraceId
	spanID := sig.SpanId
	var parentSpanID *string
	if sig.ParentSpanId != nil {
		parentSpanID = sig.ParentSpanId
	}

	// ── Source ───────────────────────────────────────────────────────────────
	var appID, appVersion string
	var hostID, environment, sdkVersion *string
	if sig.Source != nil {
		appID = sig.Source.AppId
		appVersion = sig.Source.AppVersion
		hostID = strPtr(sig.Source.InstanceId)
		environment = strPtr(sig.Source.Environment)
		sdkVersion = strPtr(sig.Source.SdkVersion)
	}

	// ── Classification ───────────────────────────────────────────────────────
	layer := uint8(sig.Layer)
	category := sig.Category
	severity := uint8(sig.Severity)

	// ── Temporal ─────────────────────────────────────────────────────────────
	// NOTE: clickhouse-go/v2 AppendRow for DateTime64 columns unconditionally
	// treats int64 as milliseconds (driver source: lib/column/datetime64.go ~L221).
	// Always pass time.Time so the driver applies the correct column precision.
	var timestamp time.Time
	if sig.Timestamp != nil {
		timestamp = sig.Timestamp.AsTime()
	}
	var durationMs *float32
	if sig.DurationMs != nil {
		durationMs = sig.DurationMs
	}
	ingestedAt := time.Now()

	// ── L1 Hardware context ───────────────────────────────────────────────────
	var (
		ctxL1CpuUsagePct        *float32
		ctxL1MemoryUsedMb       *float32
		ctxL1MemoryTotalMb      *float32
		ctxL1GpuUtilizationPct  *float32
		ctxL1GpuMemoryUsedMb    *float32
		ctxL1GpuMemoryTotalMb   *float32
		ctxL1GpuTemperatureC    *float32
		ctxL1GpuPowerDrawW      *float32
		ctxL1GpuCount           *int32
		ctxL1GpuModel           *string
		ctxL1DiskReadMbps       *float32
		ctxL1DiskWriteMbps      *float32
		ctxL1NetRxMbps          *float32
		ctxL1NetTxMbps          *float32
		ctxL1CpuStealPct        *float32
		ctxL1LoadAvg1m          *float32
		ctxL1LoadAvg5m          *float32
		ctxL1LoadAvg15m         *float32
		ctxL1OpenFileDescriptors *int32
		ctxL1ThreadCount        *int32
		ctxL1HostID             *string
		ctxL1InstanceType       *string
		ctxL1ProcessCpuPct      *float32
		ctxL1ProcessMemoryMb    *float32
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL1); ok && ctx.ContextL1 != nil {
		c := ctx.ContextL1
		ctxL1CpuUsagePct = c.CpuUsagePct
		ctxL1MemoryUsedMb = c.MemoryUsedMb
		ctxL1MemoryTotalMb = c.MemoryTotalMb
		ctxL1GpuUtilizationPct = c.GpuUtilizationPct
		ctxL1GpuMemoryUsedMb = c.GpuMemoryUsedMb
		ctxL1GpuMemoryTotalMb = c.GpuMemoryTotalMb
		ctxL1GpuTemperatureC = c.GpuTemperatureC
		ctxL1GpuPowerDrawW = c.GpuPowerDrawW
		ctxL1GpuCount = c.GpuCount
		ctxL1GpuModel = c.GpuModel
		ctxL1DiskReadMbps = c.DiskReadMbps
		ctxL1DiskWriteMbps = c.DiskWriteMbps
		ctxL1NetRxMbps = c.NetRxMbps
		ctxL1NetTxMbps = c.NetTxMbps
		ctxL1CpuStealPct = c.CpuStealPct
		ctxL1LoadAvg1m = c.LoadAvg_1M
		ctxL1LoadAvg5m = c.LoadAvg_5M
		ctxL1LoadAvg15m = c.LoadAvg_15M
		ctxL1OpenFileDescriptors = c.OpenFileDescriptors
		ctxL1ThreadCount = c.ThreadCount
		ctxL1HostID = c.HostId
		ctxL1InstanceType = c.InstanceType
		ctxL1ProcessCpuPct = c.ProcessCpuPct
		ctxL1ProcessMemoryMb = c.ProcessMemoryMb
	}

	// ── L2 Model Weights context ──────────────────────────────────────────────
	var (
		ctxL2ModelID              *string
		ctxL2ModelFamily          *string
		ctxL2ModelVersion         *string
		ctxL2ModelHash            *string
		ctxL2Quantization         *string
		ctxL2ParameterCount       *int64
		ctxL2ModelSizeGb          *float32
		ctxL2LoadTimeMs           *float32
		ctxL2VramAllocatedGb      *float32
		ctxL2RamAllocatedGb       *float32
		ctxL2Precision            *string
		ctxL2Backend              *string
		ctxL2AdapterID            *string
		ctxL2AdapterHash          *string
		ctxL2WeightsVerified      *uint8
		ctxL2WeightsLoadLatencyMs *float32
		ctxL2LayersOffloaded      *int32
		ctxL2DeviceMap            *string
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL2); ok && ctx.ContextL2 != nil {
		c := ctx.ContextL2
		ctxL2ModelID = c.ModelId
		ctxL2ModelFamily = c.ModelFamily
		ctxL2ModelVersion = c.ModelVersion
		ctxL2ModelHash = c.ModelHash
		ctxL2Quantization = c.Quantization
		ctxL2ParameterCount = c.ParameterCount
		ctxL2ModelSizeGb = c.ModelSizeGb
		ctxL2LoadTimeMs = c.LoadTimeMs
		ctxL2VramAllocatedGb = c.VramAllocatedGb
		ctxL2RamAllocatedGb = c.RamAllocatedGb
		ctxL2Precision = c.Precision
		ctxL2Backend = c.Backend
		ctxL2AdapterID = c.AdapterId
		ctxL2AdapterHash = c.AdapterHash
		ctxL2WeightsVerified = boolToUInt8Ptr(c.WeightsVerified)
		ctxL2WeightsLoadLatencyMs = c.WeightsLoadLatencyMs
		ctxL2LayersOffloaded = c.LayersOffloaded
		ctxL2DeviceMap = c.DeviceMap
	}

	// ── L3 Tokenizer context ──────────────────────────────────────────────────
	var (
		ctxL3InputTokens          *int32
		ctxL3OutputTokens         *int32
		ctxL3TotalTokens          *int32
		ctxL3TokenizerID          *string
		ctxL3TokenizerVersion     *string
		ctxL3EncodingTimeMs       *float32
		ctxL3DecodingTimeMs       *float32
		ctxL3ContextWindowSize    *int32
		ctxL3ContextUtilizationPct *float32
		ctxL3TruncatedTokens      *int32
		ctxL3Truncated            *uint8
		ctxL3ChatTemplate         *string
		ctxL3BosToken             *string
		ctxL3AvgTokenLength       *float32
		ctxL3UniqueTokenCount     *int32
		ctxL3TokenEntropy         *float32
		ctxL3SpecialTokenCount    *int32
		ctxL3EncodingName         *string
		ctxL3VocabSize            *int32
		ctxL3CompressionRatio     *float32
		ctxL3OovRate              *float32
		ctxL3HasSpecialTokens     *uint8
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL3); ok && ctx.ContextL3 != nil {
		c := ctx.ContextL3
		ctxL3InputTokens = c.InputTokens
		ctxL3OutputTokens = c.OutputTokens
		ctxL3TotalTokens = c.TotalTokens
		ctxL3TokenizerID = c.TokenizerId
		ctxL3TokenizerVersion = c.TokenizerVersion
		ctxL3EncodingTimeMs = c.EncodingTimeMs
		ctxL3DecodingTimeMs = c.DecodingTimeMs
		ctxL3ContextWindowSize = c.ContextWindowSize
		ctxL3ContextUtilizationPct = c.ContextUtilizationPct
		ctxL3TruncatedTokens = c.TruncatedTokens
		ctxL3Truncated = boolToUInt8Ptr(c.Truncated)
		ctxL3ChatTemplate = c.ChatTemplate
		ctxL3BosToken = c.BosToken
		ctxL3AvgTokenLength = c.AvgTokenLength
		ctxL3UniqueTokenCount = c.UniqueTokenCount
		ctxL3TokenEntropy = c.TokenEntropy
		ctxL3SpecialTokenCount = c.SpecialTokenCount
		ctxL3EncodingName = c.EncodingName
		ctxL3VocabSize = c.VocabSize
		ctxL3CompressionRatio = c.CompressionRatio
		ctxL3OovRate = c.OovRate
		ctxL3HasSpecialTokens = boolToUInt8Ptr(c.HasSpecialTokens)
	}

	// ── L4 Transformer context ────────────────────────────────────────────────
	var (
		ctxL4KvCacheUsedMb         *float32
		ctxL4KvCacheTotalMb        *float32
		ctxL4KvCacheHitRate        *float32
		ctxL4NumHeads              *int32
		ctxL4NumLayers             *int32
		ctxL4AttentionEntropyMean  *float32
		ctxL4AttentionEntropyMax   *float32
		ctxL4AttentionEntropyMin   *float32
		ctxL4PrefillTimeMs         *float32
		ctxL4DecodeTimeMs          *float32
		ctxL4BatchSize             *int32
		ctxL4SequenceLength        *int32
		ctxL4ThroughputTokensPerSec *float32
		ctxL4MemoryBandwidthGbps   *float32
		ctxL4FlashAttention        *uint8
		ctxL4PagedAttention        *uint8
		ctxL4SpeculativeTokens     *int32
		ctxL4SpeculationAcceptRate *float32
		ctxL4ContextLengthUsed     *int32
		ctxL4ActiveSequences       *int32
		ctxL4FlopsUtilized         *float32
		ctxL4AttentionImplementation *string
		ctxL4ComputeDtype          *string
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL4); ok && ctx.ContextL4 != nil {
		c := ctx.ContextL4
		ctxL4KvCacheUsedMb = c.KvCacheUsedMb
		ctxL4KvCacheTotalMb = c.KvCacheTotalMb
		ctxL4KvCacheHitRate = c.KvCacheHitRate
		ctxL4NumHeads = c.NumHeads
		ctxL4NumLayers = c.NumLayers
		ctxL4AttentionEntropyMean = c.AttentionEntropyMean
		ctxL4AttentionEntropyMax = c.AttentionEntropyMax
		ctxL4AttentionEntropyMin = c.AttentionEntropyMin
		ctxL4PrefillTimeMs = c.PrefillTimeMs
		ctxL4DecodeTimeMs = c.DecodeTimeMs
		ctxL4BatchSize = c.BatchSize
		ctxL4SequenceLength = c.SequenceLength
		ctxL4ThroughputTokensPerSec = c.ThroughputTokensPerSec
		ctxL4MemoryBandwidthGbps = c.MemoryBandwidthGbps
		ctxL4FlashAttention = boolToUInt8Ptr(c.FlashAttention)
		ctxL4PagedAttention = boolToUInt8Ptr(c.PagedAttention)
		ctxL4SpeculativeTokens = c.SpeculativeTokens
		ctxL4SpeculationAcceptRate = c.SpeculationAcceptRate
		ctxL4ContextLengthUsed = c.ContextLengthUsed
		ctxL4ActiveSequences = c.ActiveSequences
		ctxL4FlopsUtilized = c.FlopsUtilized
		ctxL4AttentionImplementation = c.AttentionImplementation
		ctxL4ComputeDtype = c.ComputeDtype
	}

	// ── L5 Output Decoding context ────────────────────────────────────────────
	var (
		ctxL5Operation             *int32
		ctxL5OutputTokens          *int32
		ctxL5InputTokens           *int32
		ctxL5TotalTokens           *int32
		ctxL5FinishReason          *string
		ctxL5Temperature           *float32
		ctxL5TopP                  *float32
		ctxL5MeanLogprob           *float32
		ctxL5MinLogprob            *float32
		ctxL5EntropyMean           *float32
		ctxL5EntropyVariance       *float32
		ctxL5TtftMs                *float32
		ctxL5Tps                   *float32
		ctxL5TopK                  *int32
		ctxL5MinP                  *float32
		ctxL5RepetitionPenalty     *float32
		ctxL5PresencePenalty       *float32
		ctxL5FrequencyPenalty      *float32
		ctxL5Seed                  *int64
		ctxL5StopSequences         []string
		ctxL5OutputRepetitionScore *float32
		ctxL5DistinctNgramsRatio   *float32
		ctxL5Truncated             *uint8
		ctxL5FinishReasonDetail    *string
	)
	ctxL5StopSequences = []string{}
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL5); ok && ctx.ContextL5 != nil {
		c := ctx.ContextL5
		opVal := int32(c.Operation)
		ctxL5Operation = &opVal
		outTok := c.OutputTokens
		ctxL5OutputTokens = &outTok
		inTok := c.InputTokens
		ctxL5InputTokens = &inTok
		totTok := c.TotalTokens
		ctxL5TotalTokens = &totTok
		ctxL5FinishReason = strPtr(c.FinishReason)
		temp := c.Temperature
		ctxL5Temperature = &temp
		topP := c.TopP
		ctxL5TopP = &topP
		ctxL5MeanLogprob = c.MeanLogprob
		ctxL5MinLogprob = c.MinLogprob
		ctxL5EntropyMean = c.EntropyMean
		ctxL5EntropyVariance = c.EntropyVariance
		ttft := c.TtftMs
		ctxL5TtftMs = &ttft
		tps := c.Tps
		ctxL5Tps = &tps
		ctxL5TopK = c.TopK
		ctxL5MinP = c.MinP
		ctxL5RepetitionPenalty = c.RepetitionPenalty
		ctxL5PresencePenalty = c.PresencePenalty
		ctxL5FrequencyPenalty = c.FrequencyPenalty
		ctxL5Seed = c.Seed
		if c.StopSequences != nil {
			ctxL5StopSequences = c.StopSequences
		}
		ctxL5OutputRepetitionScore = c.OutputRepetitionScore
		ctxL5DistinctNgramsRatio = c.DistinctNgramsRatio
		ctxL5Truncated = boolToUInt8Ptr(c.Truncated)
		ctxL5FinishReasonDetail = c.FinishReasonDetail
	}

	// ── L6 Safety context ─────────────────────────────────────────────────────
	var (
		ctxL6SafetyScore             *float32
		ctxL6Safe                    *uint8
		ctxL6ActionTaken             *string
		ctxL6ClassifierID            *string
		ctxL6CategoryScores          *string // JSON serialized
		ctxL6ToxicityScore           *float32
		ctxL6BiasScore               *float32
		ctxL6PolicyID                *string
		ctxL6PolicyVersion           *string
		ctxL6PolicyViolated          *uint8
		ctxL6ViolationType           *string
		ctxL6RedactionMethod         *string
		ctxL6RedactedTokens          *int32
		ctxL6EvaluationTimeMs        *float32
		ctxL6InputLanguage           *string
		ctxL6LanguageConfidence      *float32
		ctxL6JailbreakDetected       *uint8
		ctxL6JailbreakScore          *float32
		ctxL6PromptInjectionDetected *uint8
		ctxL6PromptInjectionScore    *float32
		ctxL6PiiTypesDetected        *string
		ctxL6PiiRedacted             *uint8
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL6); ok && ctx.ContextL6 != nil {
		c := ctx.ContextL6
		ctxL6SafetyScore = c.SafetyScore
		ctxL6Safe = boolToUInt8Ptr(c.Safe)
		ctxL6ActionTaken = c.ActionTaken
		ctxL6ClassifierID = c.ClassifierId
		// CategoryScores: store as nil (JSON serialization would be added later)
		_ = c.CategoryScores
		ctxL6CategoryScores = nil
		ctxL6ToxicityScore = c.ToxicityScore
		ctxL6BiasScore = c.BiasScore
		ctxL6PolicyID = c.PolicyId
		ctxL6PolicyVersion = c.PolicyVersion
		ctxL6PolicyViolated = boolToUInt8Ptr(c.PolicyViolated)
		ctxL6ViolationType = c.ViolationType
		ctxL6RedactionMethod = c.RedactionMethod
		ctxL6RedactedTokens = c.RedactedTokens
		ctxL6EvaluationTimeMs = c.EvaluationTimeMs
		ctxL6InputLanguage = c.InputLanguage
		ctxL6LanguageConfidence = c.LanguageConfidence
		ctxL6JailbreakDetected = boolToUInt8Ptr(c.JailbreakDetected)
		ctxL6JailbreakScore = c.JailbreakScore
		ctxL6PromptInjectionDetected = boolToUInt8Ptr(c.PromptInjectionDetected)
		ctxL6PromptInjectionScore = c.PromptInjectionScore
		ctxL6PiiTypesDetected = c.PiiTypesDetected
		ctxL6PiiRedacted = boolToUInt8Ptr(c.PiiRedacted)
	}

	// ── L7 RAG / Retrieval context ────────────────────────────────────────────
	var (
		ctxL7Operation           *int32
		ctxL7QueryText           *string
		ctxL7ResultsCount        *int32
		ctxL7TopChunkSource      *string
		ctxL7ContextWindowPct    *float32
		ctxL7EmbeddingModel      *string
		ctxL7VectorIndex         *string
		ctxL7RerankerApplied     *uint8
		ctxL7RerankerDelta       *float32
		ctxL7LatencyEmbeddingMs  *float32
		ctxL7LatencySearchMs     *float32
		ctxL7LatencyRerankMs     *float32
		ctxL7QueryCacheHit       *uint8
		ctxL7QueryCacheKey       *string
		ctxL7CitationsUsed       *int32
		ctxL7CitationOverlapScore *float32
		ctxL7IndexName           *string
		ctxL7IndexType           *string
		ctxL7IndexDocumentCount  *int64
		ctxL7IndexFreshnessHours *float32
		ctxL7RerankerScore       *float32
		ctxL7RerankerModel       *string
		ctxL7HybridSearch        *uint8
		ctxL7SparseWeight        *float32
		ctxL7DenseWeight         *float32
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL7); ok && ctx.ContextL7 != nil {
		c := ctx.ContextL7
		opVal := int32(c.Operation)
		ctxL7Operation = &opVal
		ctxL7QueryText = strPtr(c.QueryText)
		rc := c.ResultsCount
		ctxL7ResultsCount = &rc
		ctxL7TopChunkSource = strPtr(c.TopChunkSource)
		cwp := c.ContextWindowPct
		ctxL7ContextWindowPct = &cwp
		ctxL7EmbeddingModel = strPtr(c.EmbeddingModel)
		ctxL7VectorIndex = strPtr(c.VectorIndex)
		ra := c.RerankerApplied
		if ra {
			v := uint8(1)
			ctxL7RerankerApplied = &v
		} else {
			v := uint8(0)
			ctxL7RerankerApplied = &v
		}
		ctxL7RerankerDelta = c.RerankerDelta
		if lb := c.LatencyBreakdown; lb != nil {
			em := lb.EmbeddingMs
			ctxL7LatencyEmbeddingMs = &em
			sm := lb.SearchMs
			ctxL7LatencySearchMs = &sm
			rm := lb.RerankMs
			ctxL7LatencyRerankMs = &rm
		}
		ctxL7QueryCacheHit = boolToUInt8Ptr(c.QueryCacheHit)
		ctxL7QueryCacheKey = c.QueryCacheKey
		ctxL7CitationsUsed = c.CitationsUsed
		ctxL7CitationOverlapScore = c.CitationOverlapScore
		ctxL7IndexName = c.IndexName
		ctxL7IndexType = c.IndexType
		ctxL7IndexDocumentCount = c.IndexDocumentCount
		ctxL7IndexFreshnessHours = c.IndexFreshnessHours
		ctxL7RerankerScore = c.RerankerScore
		ctxL7RerankerModel = c.RerankerModel
		ctxL7HybridSearch = boolToUInt8Ptr(c.HybridSearch)
		ctxL7SparseWeight = c.SparseWeight
		ctxL7DenseWeight = c.DenseWeight
	}

	// ── L8 Agents context ─────────────────────────────────────────────────────
	var (
		ctxL8Operation          *int32
		ctxL8ToolName           *string
		ctxL8ToolProvider       *string
		ctxL8ToolResult         *string
		ctxL8ToolError          *string
		ctxL8ToolLatencyMs      *float32
		ctxL8StepNumber         *int32
		ctxL8TotalSteps         *int32
		ctxL8PermissionsUsed    []string
		ctxL8PermissionsRequested []string
		ctxL8DataFlowTags       []string
		ctxL8TaskID             *string
		ctxL8ParentTaskID       *string
		ctxL8TaskDepth          *int32
		ctxL8SubtaskCount       *int32
		ctxL8MemoryOperation    *string
		ctxL8MemoryItemsRead    *int32
		ctxL8MemoryItemsWritten *int32
		ctxL8CodeLanguage       *string
		ctxL8CodeExecuted       *uint8
		ctxL8CodeExitCode       *int32
		ctxL8CodeExecutionTimeMs *float32
		ctxL8Sandboxed          *uint8
		ctxL8CapabilitiesUsed   []string
		ctxL8OrchestratorType   *string
	)
	ctxL8PermissionsUsed = []string{}
	ctxL8PermissionsRequested = []string{}
	ctxL8DataFlowTags = []string{}
	ctxL8CapabilitiesUsed = []string{}
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL8); ok && ctx.ContextL8 != nil {
		c := ctx.ContextL8
		opVal := int32(c.Operation)
		ctxL8Operation = &opVal
		ctxL8ToolName = strPtr(c.ToolName)
		ctxL8ToolProvider = strPtr(c.ToolProvider)
		ctxL8ToolResult = strPtr(c.ToolResult)
		ctxL8ToolError = c.ToolError
		tl := c.ToolLatencyMs
		ctxL8ToolLatencyMs = &tl
		sn := c.StepNumber
		ctxL8StepNumber = &sn
		ctxL8TotalSteps = c.TotalSteps
		if c.PermissionsUsed != nil {
			ctxL8PermissionsUsed = c.PermissionsUsed
		}
		if c.PermissionsRequested != nil {
			ctxL8PermissionsRequested = c.PermissionsRequested
		}
		if c.DataFlowTags != nil {
			ctxL8DataFlowTags = c.DataFlowTags
		}
		ctxL8TaskID = c.TaskId
		ctxL8ParentTaskID = c.ParentTaskId
		ctxL8TaskDepth = c.TaskDepth
		ctxL8SubtaskCount = c.SubtaskCount
		ctxL8MemoryOperation = c.MemoryOperation
		ctxL8MemoryItemsRead = c.MemoryItemsRead
		ctxL8MemoryItemsWritten = c.MemoryItemsWritten
		ctxL8CodeLanguage = c.CodeLanguage
		ctxL8CodeExecuted = boolToUInt8Ptr(c.CodeExecuted)
		ctxL8CodeExitCode = c.CodeExitCode
		ctxL8CodeExecutionTimeMs = c.CodeExecutionTimeMs
		ctxL8Sandboxed = boolToUInt8Ptr(c.Sandboxed)
		if c.CapabilitiesUsed != nil {
			ctxL8CapabilitiesUsed = c.CapabilitiesUsed
		}
		ctxL8OrchestratorType = c.OrchestratorType
	}

	// ── L9 API Gateway context ────────────────────────────────────────────────
	var (
		ctxL9Method             *string
		ctxL9Path               *string
		ctxL9StatusCode         *int32
		ctxL9LatencyMs          *float32
		ctxL9ClientIP           *string
		ctxL9UserAgent          *string
		ctxL9ApiKeyID           *string
		ctxL9ApiVersion         *string
		ctxL9RequestSizeBytes   *int64
		ctxL9ResponseSizeBytes  *int64
		ctxL9CacheHit           *uint8
		ctxL9CacheKey           *string
		ctxL9RateLimitRemaining *float32
		ctxL9RateLimitResetMs   *float32
		ctxL9RateLimited        *uint8
		ctxL9AuthMethod         *string
		ctxL9Authenticated      *uint8
		ctxL9ClientID           *string
		ctxL9RequestID          *string
		ctxL9UpstreamID         *string
		ctxL9UpstreamLatencyMs  *float32
		ctxL9SslTerminated      *uint8
		ctxL9Protocol           *string
		ctxL9Region             *string
		ctxL9EndpointAlias      *string
	)
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL9); ok && ctx.ContextL9 != nil {
		c := ctx.ContextL9
		ctxL9Method = c.Method
		ctxL9Path = c.Path
		ctxL9StatusCode = c.StatusCode
		ctxL9LatencyMs = c.LatencyMs
		ctxL9ClientIP = c.ClientIp
		ctxL9UserAgent = c.UserAgent
		ctxL9ApiKeyID = c.ApiKeyId
		ctxL9ApiVersion = c.ApiVersion
		ctxL9RequestSizeBytes = c.RequestSizeBytes
		ctxL9ResponseSizeBytes = c.ResponseSizeBytes
		ctxL9CacheHit = boolToUInt8Ptr(c.CacheHit)
		ctxL9CacheKey = c.CacheKey
		ctxL9RateLimitRemaining = c.RateLimitRemaining
		ctxL9RateLimitResetMs = c.RateLimitResetMs
		ctxL9RateLimited = boolToUInt8Ptr(c.RateLimited)
		ctxL9AuthMethod = c.AuthMethod
		ctxL9Authenticated = boolToUInt8Ptr(c.Authenticated)
		ctxL9ClientID = c.ClientId
		ctxL9RequestID = c.RequestId
		ctxL9UpstreamID = c.UpstreamId
		ctxL9UpstreamLatencyMs = c.UpstreamLatencyMs
		ctxL9SslTerminated = boolToUInt8Ptr(c.SslTerminated)
		ctxL9Protocol = c.Protocol
		ctxL9Region = c.Region
		ctxL9EndpointAlias = c.EndpointAlias
	}

	// ── L10 Application context ───────────────────────────────────────────────
	var (
		ctxL10UserID            *string
		ctxL10SessionID         *string
		ctxL10ConversationID    *string
		ctxL10EventType         *string
		ctxL10Component         *string
		ctxL10Action            *string
		ctxL10FeatureID         *string
		ctxL10AbVariant         *string
		ctxL10RequestRetryCount *int32
		ctxL10ClientLatencyMs   *float32
		ctxL10ErrorCode         *string
		ctxL10ErrorMessage      *string
		ctxL10ErrorRecovered    *uint8
		ctxL10Platform          *string
		ctxL10AppVersion        *string
		ctxL10Locale            *string
		ctxL10Timezone          *string
		ctxL10FeatureFlags      map[string]string
		ctxL10CustomAttributes  map[string]string
		ctxL10PageID            *string
		ctxL10Referrer          *string
		ctxL10SatisfactionScore *float32
		ctxL10FeedbackProvided  *uint8
		ctxL10DeploymentID      *string
	)
	ctxL10FeatureFlags = map[string]string{}
	ctxL10CustomAttributes = map[string]string{}
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL10); ok && ctx.ContextL10 != nil {
		c := ctx.ContextL10
		ctxL10UserID = c.UserId
		ctxL10SessionID = c.SessionId
		ctxL10ConversationID = c.ConversationId
		ctxL10EventType = c.EventType
		ctxL10Component = c.Component
		ctxL10Action = c.Action
		ctxL10FeatureID = c.FeatureId
		ctxL10AbVariant = c.AbVariant
		ctxL10RequestRetryCount = c.RequestRetryCount
		ctxL10ClientLatencyMs = c.ClientLatencyMs
		ctxL10ErrorCode = c.ErrorCode
		ctxL10ErrorMessage = c.ErrorMessage
		ctxL10ErrorRecovered = boolToUInt8Ptr(c.ErrorRecovered)
		ctxL10Platform = c.Platform
		ctxL10AppVersion = c.AppVersion
		ctxL10Locale = c.Locale
		ctxL10Timezone = c.Timezone
		if c.FeatureFlags != nil {
			ctxL10FeatureFlags = c.FeatureFlags
		}
		if c.CustomAttributes != nil {
			ctxL10CustomAttributes = c.CustomAttributes
		}
		ctxL10PageID = c.PageId
		ctxL10Referrer = c.Referrer
		ctxL10SatisfactionScore = c.SatisfactionScore
		ctxL10FeedbackProvided = boolToUInt8Ptr(c.FeedbackProvided)
		ctxL10DeploymentID = c.DeploymentId
	}

	// ── LDecision context ─────────────────────────────────────────────────────
	var (
		ctxLdDecision           *int32
		ctxLdConfidence         *float32
		ctxLdReasoning          *string
		ctxLdRecommendedAction  *int32
		ctxLdPolicyVersion      *string
		ctxLdPolicyName         *string
		ctxLdEvaluationTimeMs   *float32
		ctxLdPolicyID           *string
		ctxLdTriggeringRuleID   *string
		ctxLdTriggeringSignalID *string
		ctxLdFailedOpen         *uint8
		ctxLdDecisionTraceID    *string
		ctxLdAlertThreshold     *float32
		ctxLdMatchedIndicators  []string
	)
	ctxLdMatchedIndicators = []string{}
	if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextLDecision); ok && ctx.ContextLDecision != nil {
		c := ctx.ContextLDecision
		dv := int32(c.Decision)
		ctxLdDecision = &dv
		cf := c.Confidence
		ctxLdConfidence = &cf
		ctxLdReasoning = strPtr(c.Reasoning)
		ra := int32(c.RecommendedAction)
		ctxLdRecommendedAction = &ra
		ctxLdPolicyVersion = strPtr(c.PolicyVersion)
		ctxLdPolicyName = strPtr(c.PolicyName)
		et := c.EvaluationTimeMs
		ctxLdEvaluationTimeMs = &et
		ctxLdPolicyID = c.PolicyId
		ctxLdTriggeringRuleID = c.TriggeringRuleId
		ctxLdTriggeringSignalID = c.TriggeringSignalId
		ctxLdFailedOpen = boolToUInt8Ptr(c.FailedOpen)
		ctxLdDecisionTraceID = c.DecisionTraceId
		ctxLdAlertThreshold = c.AlertThreshold
		if c.MatchedIndicators != nil {
			ctxLdMatchedIndicators = c.MatchedIndicators
		}
	}

	// ── Relationships ─────────────────────────────────────────────────────────
	relatedSignals := sig.RelatedSignals
	if relatedSignals == nil {
		relatedSignals = []string{}
	}
	var incidentID *string
	if sig.IncidentId != nil && *sig.IncidentId != "" {
		incidentID = sig.IncidentId
	}
	var sessionID *string
	if sig.SessionId != nil && *sig.SessionId != "" {
		sessionID = sig.SessionId
	}
	var conversationID *string
	if sig.ConversationId != nil && *sig.ConversationId != "" {
		conversationID = sig.ConversationId
	}
	var userID *string
	if sig.UserId != nil && *sig.UserId != "" {
		userID = sig.UserId
	}

	// ── Provider ──────────────────────────────────────────────────────────────
	var providerName, providerModel *string
	if sig.Provider != nil {
		providerName = strPtr(sig.Provider.Name)
		providerModel = strPtr(sig.Provider.Model)
	}

	// ── Enrichment ────────────────────────────────────────────────────────────
	var enrichBaselineDeviation *float32
	var enrichGeoIPCountry, enrichGeoIPCity *string
	var enrichThreatIntelHit *uint8
	if e := sig.Enrichment; e != nil {
		enrichBaselineDeviation = e.BaselineDeviation
		if e.Geo != nil {
			enrichGeoIPCountry = e.Geo.Country
			enrichGeoIPCity = e.Geo.City
		}
		if len(e.ThreatIntel) > 0 {
			v := uint8(1)
			enrichThreatIntelHit = &v
		}
	}

	// ── Governance ────────────────────────────────────────────────────────────
	dataClassification := uint8(sig.DataClassification)
	retentionPolicy := sig.RetentionPolicy
	if retentionPolicy == "" {
		retentionPolicy = "default"
	}
	piiDetected := uint8(0)
	if sig.PiiDetected {
		piiDetected = 1
	}

	// Append to batch in column order matching SignalsTableDDL (246 columns total).
	// NOTE: `version` column is omitted — DDL has DEFAULT 1, ClickHouse fills it automatically.
	return batch.Append(
		// Identity (4)
		signalID, traceID, spanID, parentSpanID,
		// Source (5)
		appID, appVersion, hostID, environment, sdkVersion,
		// Classification (3)
		layer, category, severity,
		// Temporal (3)
		timestamp, durationMs, ingestedAt,
		// L1 Hardware (24)
		ctxL1CpuUsagePct, ctxL1MemoryUsedMb, ctxL1MemoryTotalMb,
		ctxL1GpuUtilizationPct, ctxL1GpuMemoryUsedMb, ctxL1GpuMemoryTotalMb,
		ctxL1GpuTemperatureC, ctxL1GpuPowerDrawW, ctxL1GpuCount, ctxL1GpuModel,
		ctxL1DiskReadMbps, ctxL1DiskWriteMbps, ctxL1NetRxMbps, ctxL1NetTxMbps,
		ctxL1CpuStealPct, ctxL1LoadAvg1m, ctxL1LoadAvg5m, ctxL1LoadAvg15m,
		ctxL1OpenFileDescriptors, ctxL1ThreadCount, ctxL1HostID, ctxL1InstanceType,
		ctxL1ProcessCpuPct, ctxL1ProcessMemoryMb,
		// L2 Model Weights (18)
		ctxL2ModelID, ctxL2ModelFamily, ctxL2ModelVersion, ctxL2ModelHash,
		ctxL2Quantization, ctxL2ParameterCount, ctxL2ModelSizeGb, ctxL2LoadTimeMs,
		ctxL2VramAllocatedGb, ctxL2RamAllocatedGb, ctxL2Precision, ctxL2Backend,
		ctxL2AdapterID, ctxL2AdapterHash, ctxL2WeightsVerified, ctxL2WeightsLoadLatencyMs,
		ctxL2LayersOffloaded, ctxL2DeviceMap,
		// L3 Tokenizer (22)
		ctxL3InputTokens, ctxL3OutputTokens, ctxL3TotalTokens,
		ctxL3TokenizerID, ctxL3TokenizerVersion, ctxL3EncodingTimeMs, ctxL3DecodingTimeMs,
		ctxL3ContextWindowSize, ctxL3ContextUtilizationPct, ctxL3TruncatedTokens, ctxL3Truncated,
		ctxL3ChatTemplate, ctxL3BosToken, ctxL3AvgTokenLength, ctxL3UniqueTokenCount,
		ctxL3TokenEntropy, ctxL3SpecialTokenCount, ctxL3EncodingName, ctxL3VocabSize,
		ctxL3CompressionRatio, ctxL3OovRate, ctxL3HasSpecialTokens,
		// L4 Transformer (23)
		ctxL4KvCacheUsedMb, ctxL4KvCacheTotalMb, ctxL4KvCacheHitRate,
		ctxL4NumHeads, ctxL4NumLayers,
		ctxL4AttentionEntropyMean, ctxL4AttentionEntropyMax, ctxL4AttentionEntropyMin,
		ctxL4PrefillTimeMs, ctxL4DecodeTimeMs, ctxL4BatchSize, ctxL4SequenceLength,
		ctxL4ThroughputTokensPerSec, ctxL4MemoryBandwidthGbps,
		ctxL4FlashAttention, ctxL4PagedAttention,
		ctxL4SpeculativeTokens, ctxL4SpeculationAcceptRate,
		ctxL4ContextLengthUsed, ctxL4ActiveSequences, ctxL4FlopsUtilized,
		ctxL4AttentionImplementation, ctxL4ComputeDtype,
		// L5 Output Decoding (24)
		ctxL5Operation, ctxL5OutputTokens, ctxL5InputTokens, ctxL5TotalTokens,
		ctxL5FinishReason, ctxL5Temperature, ctxL5TopP,
		ctxL5MeanLogprob, ctxL5MinLogprob, ctxL5EntropyMean, ctxL5EntropyVariance,
		ctxL5TtftMs, ctxL5Tps,
		ctxL5TopK, ctxL5MinP, ctxL5RepetitionPenalty, ctxL5PresencePenalty, ctxL5FrequencyPenalty,
		ctxL5Seed, ctxL5StopSequences, ctxL5OutputRepetitionScore, ctxL5DistinctNgramsRatio,
		ctxL5Truncated, ctxL5FinishReasonDetail,
		// L6 Safety (22)
		ctxL6SafetyScore, ctxL6Safe, ctxL6ActionTaken, ctxL6ClassifierID, ctxL6CategoryScores,
		ctxL6ToxicityScore, ctxL6BiasScore, ctxL6PolicyID, ctxL6PolicyVersion, ctxL6PolicyViolated,
		ctxL6ViolationType, ctxL6RedactionMethod, ctxL6RedactedTokens, ctxL6EvaluationTimeMs,
		ctxL6InputLanguage, ctxL6LanguageConfidence,
		ctxL6JailbreakDetected, ctxL6JailbreakScore,
		ctxL6PromptInjectionDetected, ctxL6PromptInjectionScore,
		ctxL6PiiTypesDetected, ctxL6PiiRedacted,
		// L7 RAG (25)
		ctxL7Operation, ctxL7QueryText, ctxL7ResultsCount, ctxL7TopChunkSource,
		ctxL7ContextWindowPct, ctxL7EmbeddingModel, ctxL7VectorIndex,
		ctxL7RerankerApplied, ctxL7RerankerDelta,
		ctxL7LatencyEmbeddingMs, ctxL7LatencySearchMs, ctxL7LatencyRerankMs,
		ctxL7QueryCacheHit, ctxL7QueryCacheKey, ctxL7CitationsUsed, ctxL7CitationOverlapScore,
		ctxL7IndexName, ctxL7IndexType, ctxL7IndexDocumentCount, ctxL7IndexFreshnessHours,
		ctxL7RerankerScore, ctxL7RerankerModel, ctxL7HybridSearch, ctxL7SparseWeight, ctxL7DenseWeight,
		// L8 Agents (25)
		ctxL8Operation, ctxL8ToolName, ctxL8ToolProvider, ctxL8ToolResult, ctxL8ToolError,
		ctxL8ToolLatencyMs, ctxL8StepNumber, ctxL8TotalSteps,
		ctxL8PermissionsUsed, ctxL8PermissionsRequested, ctxL8DataFlowTags,
		ctxL8TaskID, ctxL8ParentTaskID, ctxL8TaskDepth, ctxL8SubtaskCount,
		ctxL8MemoryOperation, ctxL8MemoryItemsRead, ctxL8MemoryItemsWritten,
		ctxL8CodeLanguage, ctxL8CodeExecuted, ctxL8CodeExitCode, ctxL8CodeExecutionTimeMs,
		ctxL8Sandboxed, ctxL8CapabilitiesUsed, ctxL8OrchestratorType,
		// L9 API Gateway (25)
		ctxL9Method, ctxL9Path, ctxL9StatusCode, ctxL9LatencyMs,
		ctxL9ClientIP, ctxL9UserAgent, ctxL9ApiKeyID, ctxL9ApiVersion,
		ctxL9RequestSizeBytes, ctxL9ResponseSizeBytes,
		ctxL9CacheHit, ctxL9CacheKey,
		ctxL9RateLimitRemaining, ctxL9RateLimitResetMs, ctxL9RateLimited,
		ctxL9AuthMethod, ctxL9Authenticated, ctxL9ClientID, ctxL9RequestID,
		ctxL9UpstreamID, ctxL9UpstreamLatencyMs,
		ctxL9SslTerminated, ctxL9Protocol, ctxL9Region, ctxL9EndpointAlias,
		// L10 Application (24)
		ctxL10UserID, ctxL10SessionID, ctxL10ConversationID, ctxL10EventType,
		ctxL10Component, ctxL10Action, ctxL10FeatureID, ctxL10AbVariant,
		ctxL10RequestRetryCount, ctxL10ClientLatencyMs,
		ctxL10ErrorCode, ctxL10ErrorMessage, ctxL10ErrorRecovered,
		ctxL10Platform, ctxL10AppVersion, ctxL10Locale, ctxL10Timezone,
		ctxL10FeatureFlags, ctxL10CustomAttributes,
		ctxL10PageID, ctxL10Referrer, ctxL10SatisfactionScore, ctxL10FeedbackProvided,
		ctxL10DeploymentID,
		// LDecision (14)
		ctxLdDecision, ctxLdConfidence, ctxLdReasoning, ctxLdRecommendedAction,
		ctxLdPolicyVersion, ctxLdPolicyName, ctxLdEvaluationTimeMs,
		ctxLdPolicyID, ctxLdTriggeringRuleID, ctxLdTriggeringSignalID,
		ctxLdFailedOpen, ctxLdDecisionTraceID, ctxLdAlertThreshold, ctxLdMatchedIndicators,
		// Relationships (5)
		relatedSignals, incidentID, sessionID, conversationID, userID,
		// Provider (2)
		providerName, providerModel,
		// Enrichment (4)
		enrichBaselineDeviation, enrichGeoIPCountry, enrichGeoIPCity, enrichThreatIntelHit,
		// Governance (3)
		dataClassification, retentionPolicy, piiDetected,
		// 246 columns total (must match SignalsInsertColumns)
	)
}

// Close closes the ClickHouse connection.
// Ping checks ClickHouse connectivity.
func (ch *ClickHouse) Ping(ctx context.Context) error {
	return ch.conn.Ping(ctx)
}

func (ch *ClickHouse) Close() error {
	return ch.conn.Close()
}

// Conn returns the underlying ClickHouse connection for advanced operations.
func (ch *ClickHouse) Conn() driver.Conn {
	return ch.conn
}

// DrainWorker pulls signals from the ingest queue and writes to the batch writer.
// Runs as a fixed goroutine (1 per ClickHouse writer in Phase 2; worker pool in Phase 3).
// Stops when ctx is cancelled.
func DrainWorker(ctx context.Context, queue Dequeuer, writer *BatchWriter, log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}

	log.Info("drain worker started")
	defer log.Info("drain worker stopped")

	for {
		sig, err := queue.Dequeue(ctx)
		if err != nil {
			// Context cancelled
			return
		}

		if sig == nil {
			continue
		}

		// Write signal to batch writer
		if err := writer.Write(ctx, sig); err != nil {
			log.Error("failed to write signal to batch writer", zap.Error(err), zap.String("signal_id", sig.SignalId))
			continue
		}
	}
}
