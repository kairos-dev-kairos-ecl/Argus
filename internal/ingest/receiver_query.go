package ingest

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/detection/engine"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/argusxdr/argus/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// QueryHandler implements the query API (GET /v1/signals).
// STORE-06 requirement: cursor-based pagination for ClickHouse signal queries.
type QueryHandler struct {
	ch      *storage.ClickHouse
	metrics *metrics.HTTP
	log     *zap.Logger
	store   *engine.RuleStore // may be nil if detection engine not wired
}

// NewQueryHandler creates a new QueryHandler.
func NewQueryHandler(ch *storage.ClickHouse, httpMetrics *metrics.HTTP, log *zap.Logger) *QueryHandler {
	if log == nil {
		log = zap.NewNop()
	}
	if httpMetrics == nil {
		httpMetrics = &metrics.HTTP{}
	}
	return &QueryHandler{
		ch:      ch,
		metrics: httpMetrics,
		log:     log,
	}
}

// SetRuleStore wires the in-memory rule store for rule management handlers.
func (h *QueryHandler) SetRuleStore(s *engine.RuleStore) {
	h.store = s
}

// RegisterRoutes mounts query API routes onto the chi mux.
func (h *QueryHandler) RegisterRoutes(mux *chi.Mux) {
	// Tier 1: Core signal query routes
	mux.Get("/v1/signals", h.handleGetSignals)
	mux.Get("/v1/schema/signals", h.handleGetSignalSchema)
	mux.Get("/api/v1/layers/status", h.handleGetLayerStatus)
	mux.Get("/api/v1/traces/{traceId}", h.handleGetTrace)
	mux.Post("/api/v1/query", h.handlePostQuery)

	// Tier 2: Rules
	mux.Get("/api/v1/rules", h.handleListRules)
	mux.Post("/api/v1/rules", h.handleCreateRule)
	mux.Get("/api/v1/rules/{id}", h.handleGetRule)
	mux.Put("/api/v1/rules/{id}", h.handleUpdateRule)
	mux.Delete("/api/v1/rules/{id}", h.handleDeleteRule)
	mux.Post("/api/v1/rules/validate", h.handleValidateRule)
	mux.Post("/api/v1/rules/test", h.handleTestRule)

	// Tier 2: Alerts
	mux.Get("/api/v1/alerts", h.handleListAlerts)
	mux.Get("/api/v1/alerts/{id}", h.handleGetAlert)
	mux.Post("/api/v1/alerts/{id}/acknowledge", h.handleAcknowledgeAlert)

	// Tier 2: Incidents
	mux.Get("/api/v1/incidents", h.handleListIncidents)
	mux.Get("/api/v1/incidents/{id}", h.handleGetIncident)
	mux.Post("/api/v1/incidents/{id}/acknowledge", h.handleAcknowledgeIncident)
	mux.Post("/api/v1/incidents/{id}/resolve", h.handleResolveIncident)

	// Tier 3: Auth
	mux.Post("/api/v1/auth/login", h.handleLogin)
	mux.Post("/api/v1/auth/refresh", h.handleRefreshToken)
	mux.Post("/api/v1/auth/logout", h.handleLogout)
	mux.Post("/api/v1/auth/setup", h.handleSetup)

	// Tier 3: Users
	mux.Get("/api/v1/users", h.handleListUsers)
	mux.Post("/api/v1/users", h.handleCreateUser)

	// Tier 3: Apps
	mux.Get("/api/v1/apps", h.handleListApps)
	mux.Post("/api/v1/apps", h.handleCreateApp)
	mux.Get("/api/v1/apps/{id}/key", h.handleGetAppKey)
	mux.Post("/api/v1/apps/{id}/key/rotate", h.handleRotateAppKey)

	// Tier 3: Audit
	mux.Get("/api/v1/audit", h.handleListAuditLog)
}

// QueryResponse is the JSON response for GET /v1/signals.
type QueryResponse struct {
	Signals   []*v1.ArgusSignal `json:"signals"`
	NextCursor string           `json:"next_cursor,omitempty"`
	TotalHint int64            `json:"total_hint"`
}

// ErrorResponse is a JSON error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// handleGetSignals handles GET /v1/signals with cursor pagination.
//
// Query parameters:
//   app_id      string    REQUIRED  Filter by app_id
//   layer       int       optional  Layer enum (1-10)
//   category    string    optional  Exact category match
//   severity    int       optional  Minimum severity (1-5)
//   start       RFC3339   optional  Timestamp >= start
//   end         RFC3339   optional  Timestamp <= end
//   cursor      string    optional  Pagination cursor (base64-encoded)
//   limit       int       optional  Page size (default 100, max 1000)
//
// Cursor format: base64({last_timestamp_ns}:{last_signal_id})
// The cursor is opaque to the client — it encodes the last result's timestamp and signal_id
// for efficient keyset pagination.
func (h *QueryHandler) handleGetSignals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	// P3: Graceful degradation — return 503 if ClickHouse is unavailable
	if h.ch == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage unavailable"})
		return
	}

	// Parse query parameters
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "app_id is required"})
		h.log.Warn("query missing app_id")
		return
	}

	// Layer: optional, 0 means no filter
	var layer int32 = 0
	if layerStr := r.URL.Query().Get("layer"); layerStr != "" {
		l, err := strconv.ParseInt(layerStr, 10, 32)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid layer"})
			return
		}
		layer = int32(l)
	}

	// Category: optional
	category := r.URL.Query().Get("category")

	// Severity: optional, 0 means no minimum
	var severity int32 = 0
	if sevStr := r.URL.Query().Get("severity"); sevStr != "" {
		s, err := strconv.ParseInt(sevStr, 10, 32)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid severity"})
			return
		}
		severity = int32(s)
	}

	// Timestamp range: optional
	var startTs, endTs *time.Time
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		st, err := time.Parse(time.RFC3339Nano, startStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid start timestamp"})
			return
		}
		startTs = &st
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		et, err := time.Parse(time.RFC3339Nano, endStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid end timestamp"})
			return
		}
		endTs = &et
	}

	// Limit: optional, default 100, max 1000
	limit := int64(100)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		l, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid limit"})
			return
		}
		if l < 1 {
			l = 1
		}
		if l > 1000 {
			l = 1000
		}
		limit = l
	}

	// Cursor: optional
	var cursorTs *time.Time
	var cursorID string
	if cursorStr := r.URL.Query().Get("cursor"); cursorStr != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursorStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid cursor"})
			return
		}

		parts := strings.Split(string(decoded), ":")
		if len(parts) != 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid cursor format"})
			return
		}

		ts, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid cursor timestamp"})
			return
		}
		cursorTs = &ts
		cursorID = parts[1]
	}

	// Build ClickHouse query with named parameters to prevent SQL injection
	query, queryArgs := buildSignalQuery(appID, layer, category, severity, startTs, endTs, cursorTs, cursorID, limit+1)
	h.log.Debug("executing query", zap.String("query", query))

	// Execute query
	rows, err := h.ch.Conn().Query(ctx, query, queryArgs...)
	if err != nil {
		h.log.Error("query failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("query failed: %v", err)})
		return
	}
	defer rows.Close()

	// Unmarshal rows into ArgusSignal structs
	signals := []*v1.ArgusSignal{}
	for rows.Next() {
		sig, err := scanSignalRow(rows)
		if err != nil {
			h.log.Error("failed to scan signal row", zap.Error(err))
			continue
		}
		signals = append(signals, sig)
	}

	if err := rows.Err(); err != nil {
		h.log.Error("row iteration failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("query failed: %v", err)})
		return
	}

	// Determine if there's a next page
	var nextCursor string
	if int64(len(signals)) > limit {
		// Trim to limit and encode next cursor
		signals = signals[:limit]
		lastSignal := signals[len(signals)-1]
		if lastSignal.Timestamp != nil {
			cursorData := lastSignal.Timestamp.AsTime().Format(time.RFC3339Nano) + ":" + lastSignal.SignalId
			nextCursor = base64.StdEncoding.EncodeToString([]byte(cursorData))
		}
	}

	// Response
	response := QueryResponse{
		Signals:    signals,
		NextCursor: nextCursor,
		TotalHint:  int64(len(signals)),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	duration := time.Since(startTime).Seconds()
	h.log.Debug("query completed", zap.Float64("duration_seconds", duration), zap.Int("signals", len(signals)))
}

// SchemaColumn represents a single column in the schema
type SchemaColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// SchemaResponse is the JSON response for GET /api/v1/schema/signals
type SchemaResponse struct {
	Columns []SchemaColumn `json:"columns"`
}

// handleGetSignalSchema handles GET /v1/schema/signals and returns the signal table schema.
func (h *QueryHandler) handleGetSignalSchema(w http.ResponseWriter, r *http.Request) {
	schema := SchemaResponse{
		Columns: []SchemaColumn{
			// Identity
			{Name: "signal_id", Type: "String", Nullable: false},
			{Name: "trace_id", Type: "String", Nullable: false},
			{Name: "span_id", Type: "String", Nullable: false},
			{Name: "parent_span_id", Type: "String", Nullable: true},

			// Classification
			{Name: "layer", Type: "UInt8", Nullable: false},
			{Name: "category", Type: "String", Nullable: false},
			{Name: "severity", Type: "UInt8", Nullable: false},

			// Temporal
			{Name: "timestamp", Type: "DateTime64(9)", Nullable: false},
			{Name: "duration_ms", Type: "Float64", Nullable: true},
			{Name: "ingested_at", Type: "DateTime64(3)", Nullable: false},

			// Source/App
			{Name: "app_id", Type: "String", Nullable: false},
			{Name: "app_version", Type: "String", Nullable: false},
			{Name: "sdk_version", Type: "String", Nullable: false},
			{Name: "environment", Type: "String", Nullable: true},
			{Name: "host_id", Type: "String", Nullable: true},

			// Provider
			{Name: "provider_name", Type: "String", Nullable: true},
			{Name: "provider_model", Type: "String", Nullable: true},

			// Relationships
			{Name: "related_signals", Type: "Array(String)", Nullable: true},
			{Name: "incident_id", Type: "String", Nullable: true},
			{Name: "session_id", Type: "String", Nullable: true},
			{Name: "conversation_id", Type: "String", Nullable: true},
			{Name: "user_id", Type: "String", Nullable: true},

			// Data governance
			{Name: "data_classification", Type: "UInt8", Nullable: false},
			{Name: "retention_policy", Type: "String", Nullable: false},
			{Name: "pii_detected", Type: "UInt8", Nullable: false},

			// Layer contexts (L1-L10)
			{Name: "ctx_l1_cpu_percent", Type: "Float32", Nullable: true},
			{Name: "ctx_l1_memory_used_mb", Type: "Float32", Nullable: true},
			{Name: "ctx_l1_gpu_utilization_pct", Type: "Float32", Nullable: true},
			{Name: "ctx_l2_model_id", Type: "String", Nullable: true},
			{Name: "ctx_l2_model_hash", Type: "String", Nullable: true},
			{Name: "ctx_l2_quantization", Type: "String", Nullable: true},
			{Name: "ctx_l3_input_token_count", Type: "Int64", Nullable: true},
			{Name: "ctx_l3_output_token_count", Type: "Int64", Nullable: true},
			{Name: "ctx_l3_truncated", Type: "UInt8", Nullable: true},
			{Name: "ctx_l4_attention_entropy", Type: "Float32", Nullable: true},
			{Name: "ctx_l4_kv_cache_hit_rate", Type: "Float32", Nullable: true},
			{Name: "ctx_l5_mean_logprob", Type: "Float32", Nullable: true},
			{Name: "ctx_l5_top_logprob", Type: "Float32", Nullable: true},
			{Name: "ctx_l5_finish_reason", Type: "String", Nullable: true},
			{Name: "ctx_l6_safety_score", Type: "Float32", Nullable: true},
			{Name: "ctx_l6_policy_violated", Type: "String", Nullable: true},
			{Name: "ctx_l6_action_taken", Type: "String", Nullable: true},
			{Name: "ctx_l7_query_text", Type: "String", Nullable: true},
			{Name: "ctx_l7_retrieved_count", Type: "Int64", Nullable: true},
			{Name: "ctx_l7_top_score", Type: "Float32", Nullable: true},
			{Name: "ctx_l7_collection_name", Type: "String", Nullable: true},
			{Name: "ctx_l8_tool_name", Type: "String", Nullable: true},
			{Name: "ctx_l8_tool_input_hash", Type: "String", Nullable: true},
			{Name: "ctx_l8_agent_step", Type: "Int64", Nullable: true},
			{Name: "ctx_l9_method", Type: "String", Nullable: true},
			{Name: "ctx_l9_path", Type: "String", Nullable: true},
			{Name: "ctx_l9_status_code", Type: "String", Nullable: true},
			{Name: "ctx_l9_latency_ms", Type: "Float32", Nullable: true},
			{Name: "ctx_l10_event_type", Type: "String", Nullable: true},
			{Name: "ctx_l10_component", Type: "String", Nullable: true},

			// Enrichment
			{Name: "enrich_baseline_deviation", Type: "Float32", Nullable: true},
			{Name: "enrich_geoip_country", Type: "String", Nullable: true},
			{Name: "enrich_geoip_city", Type: "String", Nullable: true},
			{Name: "enrich_threat_intel_hit", Type: "Int64", Nullable: true},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(schema)
}

// LayerStatusResponse is the JSON response for GET /api/v1/layers/status.
type LayerStatusResponse struct {
	Layers []LayerStatus `json:"layers"`
}

// LayerStatus represents the health/activity status of one L1-L10 layer.
type LayerStatus struct {
	Layer        int32  `json:"layer"`
	Name         string `json:"name"`
	SignalCount  int64  `json:"signal_count"`
	LastSeenAt   string `json:"last_seen_at,omitempty"`
	Status       string `json:"status"` // "active", "idle", "unknown"
}

var layerNames = map[int32]string{
	1: "Hardware", 2: "Model", 3: "Tokenizer", 4: "Transformer",
	5: "Output", 6: "Safety", 7: "RAG", 8: "Agents", 9: "API", 10: "Application",
}

// handleGetLayerStatus handles GET /api/v1/layers/status.
// Returns signal activity per layer for the past 5 minutes.
func (h *QueryHandler) handleGetLayerStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// P3: degraded mode
	if h.ch == nil {
		resp := LayerStatusResponse{Layers: make([]LayerStatus, 0, 10)}
		for i := int32(1); i <= 10; i++ {
			resp.Layers = append(resp.Layers, LayerStatus{Layer: i, Name: layerNames[i], Status: "unknown"})
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	ctx := r.Context()
	query := `
		SELECT layer, count() AS cnt, max(timestamp) AS last_seen
		FROM signals
		WHERE timestamp >= now() - INTERVAL 5 MINUTE
		GROUP BY layer
		ORDER BY layer`

	rows, err := h.ch.Conn().Query(ctx, query)
	if err != nil {
		h.log.Warn("layer status query failed", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage query failed"})
		return
	}
	defer rows.Close()

	counts := map[int32]int64{}
	lastSeen := map[int32]string{}
	for rows.Next() {
		var layer uint8
		var cnt int64
		var ls time.Time
		if err := rows.Scan(&layer, &cnt, &ls); err == nil {
			counts[int32(layer)] = cnt
			lastSeen[int32(layer)] = ls.UTC().Format(time.RFC3339)
		}
	}
	if err := rows.Err(); err != nil {
		h.log.Warn("layer status row iteration failed", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage query failed"})
		return
	}

	resp := LayerStatusResponse{Layers: make([]LayerStatus, 0, 10)}
	for i := int32(1); i <= 10; i++ {
		status := "idle"
		if counts[i] > 0 {
			status = "active"
		}
		resp.Layers = append(resp.Layers, LayerStatus{
			Layer:       i,
			Name:        layerNames[i],
			SignalCount: counts[i],
			LastSeenAt:  lastSeen[i],
			Status:      status,
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// TraceResponse is the JSON response for GET /api/v1/traces/{traceId}.
type TraceResponse struct {
	TraceID string            `json:"trace_id"`
	Signals []*v1.ArgusSignal `json:"signals"`
}

// handleGetTrace handles GET /api/v1/traces/{traceId}.
// Returns all signals for a trace ordered by timestamp.
func (h *QueryHandler) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.ch == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage unavailable"})
		return
	}

	traceID := chi.URLParam(r, "traceId")
	if traceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "traceId is required"})
		return
	}

	ctx := r.Context()
	const traceQuery = `
SELECT
    signal_id, trace_id, span_id, parent_span_id,
    layer, category, severity,
    timestamp, duration_ms, ingested_at,
    app_id, app_version, sdk_version, environment, host_id,
    provider_name, provider_model,
    related_signals, incident_id, session_id, conversation_id, user_id,
    data_classification, retention_policy, pii_detected,
    ctx_l1_cpu_percent, ctx_l1_memory_used_mb, ctx_l1_gpu_utilization_pct,
    ctx_l2_model_id, ctx_l2_model_hash, ctx_l2_quantization,
    ctx_l3_input_token_count, ctx_l3_output_token_count, ctx_l3_truncated,
    ctx_l4_attention_entropy, ctx_l4_kv_cache_hit_rate,
    ctx_l5_mean_logprob, ctx_l5_top_logprob, ctx_l5_finish_reason,
    ctx_l6_safety_score, ctx_l6_policy_violated, ctx_l6_action_taken,
    ctx_l7_query_text, ctx_l7_retrieved_count, ctx_l7_top_score, ctx_l7_collection_name,
    ctx_l8_tool_name, ctx_l8_tool_input_hash, ctx_l8_agent_step,
    ctx_l9_method, ctx_l9_path, ctx_l9_status_code, ctx_l9_latency_ms,
    ctx_l10_event_type, ctx_l10_component,
    enrich_baseline_deviation, enrich_geoip_country, enrich_geoip_city, enrich_threat_intel_hit
FROM signals FINAL
WHERE trace_id = {trace_id:String}
ORDER BY timestamp ASC
LIMIT 1000`

	rows, err := h.ch.Conn().Query(ctx, traceQuery, clickhouse.Named("trace_id", traceID))
	if err != nil {
		h.log.Error("trace query failed", zap.String("trace_id", traceID), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "query failed"})
		return
	}
	defer rows.Close()

	signals := make([]*v1.ArgusSignal, 0)
	for rows.Next() {
		sig, err := scanSignalRow(rows)
		if err != nil {
			h.log.Warn("failed to scan trace signal row", zap.Error(err))
			continue
		}
		signals = append(signals, sig)
	}
	if err := rows.Err(); err != nil {
		h.log.Error("trace row iteration failed", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "query iteration failed"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TraceResponse{TraceID: traceID, Signals: signals})
}

// scanSignalRow scans a single ClickHouse row (from the signals SELECT query) into an ArgusSignal.
// The SELECT column order must match the order used in buildSignalQuery and traceQuery.
func scanSignalRow(rows driver.Rows) (*v1.ArgusSignal, error) {
	var (
		// Identity
		signalID, traceID, spanID string
		parentSpanID              sql.NullString

		// Classification
		layer, severity uint8
		category        string

		// Temporal
		timestamp  time.Time
		durationMs sql.NullFloat64
		ingestedAt time.Time

		// Source/App info
		appID, appVersion, sdkVersion string
		environment, hostID           sql.NullString

		// Provider
		providerName, providerModel sql.NullString

		// Related signals
		relatedSignalsStr []string

		// Relationships
		incidentID, sessionID, conversationID, userID sql.NullString

		// Data classification
		dataClassification uint8
		retentionPolicy    string
		piiDetected        uint8

		// Layer contexts (not yet fully implemented in proto, scan and ignore)
		ctxL1CPU, ctxL1Memory, ctxL1GPU                sql.NullFloat64
		ctxL2ModelID, ctxL2ModelHash, ctxL2Quantization sql.NullString
		ctxL3InputTokens, ctxL3OutputTokens, ctxL3Truncated sql.NullInt64
		ctxL4AttentionEntropy, ctxL4KVCacheHitRate      sql.NullFloat64
		ctxL5MeanLogprob, ctxL5TopLogprob               sql.NullFloat64
		ctxL5FinishReason                               sql.NullString
		ctxL6SafetyScore                                sql.NullFloat64
		ctxL6PolicyViolated, ctxL6ActionTaken           sql.NullString
		ctxL7QueryText                                  sql.NullString
		ctxL7RetrievedCount                             sql.NullInt64
		ctxL7TopScore                                   sql.NullFloat64
		ctxL7CollectionName                             sql.NullString
		ctxL8ToolName, ctxL8ToolInputHash               sql.NullString
		ctxL8AgentStep                                  sql.NullInt64
		ctxL9Method, ctxL9Path, ctxL9StatusCode         sql.NullString
		ctxL9LatencyMs                                  sql.NullFloat64
		ctxL10EventType, ctxL10Component                sql.NullString

		// Enrichment
		enrichBaselineDeviation          sql.NullFloat64
		enrichGeoipCountry, enrichGeoipCity sql.NullString
		enrichThreatIntelHit             sql.NullInt64
	)

	err := rows.Scan(
		&signalID, &traceID, &spanID, &parentSpanID,
		&layer, &category, &severity,
		&timestamp, &durationMs, &ingestedAt,
		&appID, &appVersion, &sdkVersion, &environment, &hostID,
		&providerName, &providerModel,
		&relatedSignalsStr, &incidentID, &sessionID, &conversationID, &userID,
		&dataClassification, &retentionPolicy, &piiDetected,
		&ctxL1CPU, &ctxL1Memory, &ctxL1GPU,
		&ctxL2ModelID, &ctxL2ModelHash, &ctxL2Quantization,
		&ctxL3InputTokens, &ctxL3OutputTokens, &ctxL3Truncated,
		&ctxL4AttentionEntropy, &ctxL4KVCacheHitRate,
		&ctxL5MeanLogprob, &ctxL5TopLogprob, &ctxL5FinishReason,
		&ctxL6SafetyScore, &ctxL6PolicyViolated, &ctxL6ActionTaken,
		&ctxL7QueryText, &ctxL7RetrievedCount, &ctxL7TopScore, &ctxL7CollectionName,
		&ctxL8ToolName, &ctxL8ToolInputHash, &ctxL8AgentStep,
		&ctxL9Method, &ctxL9Path, &ctxL9StatusCode, &ctxL9LatencyMs,
		&ctxL10EventType, &ctxL10Component,
		&enrichBaselineDeviation, &enrichGeoipCountry, &enrichGeoipCity, &enrichThreatIntelHit,
	)
	if err != nil {
		return nil, err
	}

	// Build ArgusSignal from scanned values.
	// Note: Some schema columns don't map to proto fields yet (layer contexts, detailed enrichment).
	// This basic implementation returns core signal data; enrichment mapping requires proto updates.

	src := &v1.Source{
		AppId:       appID,
		AppVersion:  appVersion,
		SdkVersion:  sdkVersion,
		Environment: environment.String,
		InstanceId:  hostID.String,
	}

	var prov *v1.Provider
	if providerName.Valid || providerModel.Valid {
		prov = &v1.Provider{
			Name:  providerName.String,
			Model: providerModel.String,
		}
	}

	var enrich *v1.Enrichment
	if enrichBaselineDeviation.Valid || enrichGeoipCountry.Valid || enrichGeoipCity.Valid || enrichThreatIntelHit.Valid {
		enrich = &v1.Enrichment{
			BaselineDeviation: func() *float32 {
				if enrichBaselineDeviation.Valid {
					v := float32(enrichBaselineDeviation.Float64)
					return &v
				}
				return nil
			}(),
			RiskScore:   nil, // Not available in schema yet
			ThreatIntel: nil, // Would require separate query/parsing
		}
		if enrichGeoipCountry.Valid || enrichGeoipCity.Valid {
			enrich.Geo = &v1.GeoData{
				Country: func() *string {
					if enrichGeoipCountry.Valid {
						return &enrichGeoipCountry.String
					}
					return nil
				}(),
				City: func() *string {
					if enrichGeoipCity.Valid {
						return &enrichGeoipCity.String
					}
					return nil
				}(),
			}
		}
	}

	signal := &v1.ArgusSignal{
		SignalId:     signalID,
		TraceId:      traceID,
		SpanId:       spanID,
		ParentSpanId: func() *string { if parentSpanID.Valid { return &parentSpanID.String }; return nil }(),
		Source:       src,
		Layer:        v1.Layer(layer),
		Category:     category,
		Severity:     v1.Severity(severity),
		Timestamp:    timestamppb.New(timestamp),
		IngestedAt:   timestamppb.New(ingestedAt),
		RelatedSignals: relatedSignalsStr,
		IncidentId:     func() *string { if incidentID.Valid { return &incidentID.String }; return nil }(),
		SessionId:      func() *string { if sessionID.Valid { return &sessionID.String }; return nil }(),
		ConversationId: func() *string { if conversationID.Valid { return &conversationID.String }; return nil }(),
		UserId:         func() *string { if userID.Valid { return &userID.String }; return nil }(),
		Provider:       prov,
		Enrichment:     enrich,
		DataClassification: v1.DataClassification(dataClassification),
		RetentionPolicy:    retentionPolicy,
		PiiDetected:        piiDetected != 0,
	}

	// Set optional duration
	if durationMs.Valid {
		v := float32(durationMs.Float64)
		signal.DurationMs = &v
	}

	// TODO: Layer contexts (L1-L10) are defined in schema but not yet implemented in proto.
	// Once proto context messages are fully defined, add mapping logic here.
	// Scanned values: ctxL1-L10 fields above

	return signal, nil
}

// QueryRequest is the JSON body for POST /api/v1/query.
type QueryRequest struct {
	SQL   string `json:"sql"`
	Limit int    `json:"limit"` // default 1000, max 5000
}

// QueryResultResponse is the JSON response for POST /api/v1/query.
type QueryResultResponse struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int             `json:"row_count"`
}

// ddlPatterns lists forbidden SQL statement prefixes (DDL and mutating DML).
var ddlPatterns = []string{
	"DROP", "DELETE", "INSERT", "UPDATE", "ALTER", "TRUNCATE",
	"CREATE", "REPLACE", "RENAME", "OPTIMIZE", "SYSTEM",
}

// handlePostQuery handles POST /api/v1/query.
// Executes a read-only SQL query against ClickHouse with safety checks.
func (h *QueryHandler) handlePostQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse and validate request first (before storage check)
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid JSON body"})
		return
	}
	if req.SQL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "sql is required"})
		return
	}

	// Safety: block DDL and mutating statements (before storage availability check)
	sqlUpper := strings.ToUpper(strings.TrimSpace(req.SQL))
	for _, pattern := range ddlPatterns {
		if strings.HasPrefix(sqlUpper, pattern) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: fmt.Sprintf("DDL and mutating statements are not allowed: %s", pattern),
			})
			return
		}
	}

	// Storage availability check (after request validation)
	if h.ch == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "storage unavailable"})
		return
	}

	// Enforce row limit
	limit := req.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	if !strings.Contains(sqlUpper, "LIMIT") {
		req.SQL = fmt.Sprintf("%s LIMIT %d", req.SQL, limit)
	}

	// Execute with 30s timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	rows, err := h.ch.Conn().Query(ctx, req.SQL)
	if err != nil {
		h.log.Warn("user query failed", zap.String("sql", req.SQL), zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	// Get column names
	colTypes := rows.ColumnTypes()
	columns := make([]string, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ct.Name()
	}

	// Scan all rows
	var resultRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		resultRows = append(resultRows, values)
	}
	if err := rows.Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "scan error: " + err.Error()})
		return
	}

	if resultRows == nil {
		resultRows = [][]interface{}{}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QueryResultResponse{
		Columns:  columns,
		Rows:     resultRows,
		RowCount: len(resultRows),
	})
}

// buildSignalQuery constructs a ClickHouse query for signals using named parameters
// to prevent SQL injection for all user-supplied string values.
// Uses FINAL modifier for ReplacingMergeTree deduplication.
// Returns the query string and the slice of clickhouse.Named args to pass to Query().
func buildSignalQuery(appID string, layer int32, category string, severity int32, startTs, endTs, cursorTs *time.Time, cursorID string, limit int64) (string, []interface{}) {
	// Base query with FINAL for dedup. app_id is always required and uses a named param.
	query := `
SELECT
	signal_id, trace_id, span_id, parent_span_id,
	layer, category, severity,
	timestamp, duration_ms, ingested_at,
	app_id, app_version, sdk_version, environment, host_id,
	provider_name, provider_model,
	related_signals, incident_id, session_id, conversation_id, user_id,
	data_classification, retention_policy, pii_detected,
	ctx_l1_cpu_percent, ctx_l1_memory_used_mb, ctx_l1_gpu_utilization_pct,
	ctx_l2_model_id, ctx_l2_model_hash, ctx_l2_quantization,
	ctx_l3_input_token_count, ctx_l3_output_token_count, ctx_l3_truncated,
	ctx_l4_attention_entropy, ctx_l4_kv_cache_hit_rate,
	ctx_l5_mean_logprob, ctx_l5_top_logprob, ctx_l5_finish_reason,
	ctx_l6_safety_score, ctx_l6_policy_violated, ctx_l6_action_taken,
	ctx_l7_query_text, ctx_l7_retrieved_count, ctx_l7_top_score, ctx_l7_collection_name,
	ctx_l8_tool_name, ctx_l8_tool_input_hash, ctx_l8_agent_step,
	ctx_l9_method, ctx_l9_path, ctx_l9_status_code, ctx_l9_latency_ms,
	ctx_l10_event_type, ctx_l10_component,
	enrich_baseline_deviation, enrich_geoip_country, enrich_geoip_city, enrich_threat_intel_hit
FROM signals FINAL
WHERE app_id = {app_id:String}`

	args := []interface{}{clickhouse.Named("app_id", appID)}

	// Numeric params (layer, severity) are safe to interpolate — parsed from integers, no user string injection.
	if layer > 0 {
		query += ` AND layer = ` + strconv.FormatInt(int64(layer), 10)
	}

	if category != "" {
		query += ` AND category = {category:String}`
		args = append(args, clickhouse.Named("category", category))
	}

	if severity > 0 {
		query += ` AND severity >= ` + strconv.FormatInt(int64(severity), 10)
	}

	// Timestamps are server-generated RFC3339Nano strings — not user-supplied raw strings.
	if startTs != nil {
		query += ` AND timestamp >= parseDatetime64BestEffort('` + startTs.Format(time.RFC3339Nano) + `')`
	}

	if endTs != nil {
		query += ` AND timestamp <= parseDatetime64BestEffort('` + endTs.Format(time.RFC3339Nano) + `')`
	}

	// Keyset pagination: (timestamp, signal_id) > (last_ts, last_id).
	// cursorID is user-supplied and uses a named param; cursorTs is server-formatted.
	if cursorTs != nil && cursorID != "" {
		query += ` AND (timestamp, signal_id) > (parseDatetime64BestEffort('` + cursorTs.Format(time.RFC3339Nano) + `'), {cursor_id:String})`
		args = append(args, clickhouse.Named("cursor_id", cursorID))
	}

	// Order and limit — limit is an int64, safe to interpolate.
	query += `
ORDER BY timestamp ASC, signal_id ASC
LIMIT ` + strconv.FormatInt(limit, 10)

	return query, args
}
