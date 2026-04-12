package ingest

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/go-chi/chi/v5"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OTLPReceiver handles OTLP ingest endpoints.
type OTLPReceiver struct {
	queue   *Queue
	metrics *metrics.Ingest
	log     *zap.Logger
}

// NewOTLPReceiver creates a new OTLP ingest receiver.
func NewOTLPReceiver(queue *Queue, metrics *metrics.Ingest, log *zap.Logger) *OTLPReceiver {
	if log == nil {
		log = zap.NewNop()
	}
	return &OTLPReceiver{
		queue:   queue,
		metrics: metrics,
		log:     log,
	}
}

// RegisterRoutes mounts OTLP endpoints on chi mux.
// Both endpoints accept:
//   Content-Type: application/x-protobuf → binary protobuf
//   Content-Type: application/json       → JSON OTLP
func (r *OTLPReceiver) RegisterRoutes(mux *chi.Mux) {
	mux.Post("/v1/traces", http.HandlerFunc(r.handleTraces))
	mux.Post("/v1/metrics", http.HandlerFunc(r.handleMetrics))
}

// handleTraces handles POST /v1/traces (OTLP ExportTraceServiceRequest).
func (r *OTLPReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	// Read request body (no strict size limit for OTLP like HTTP)
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		r.log.Error("failed to read request body", zap.Error(err))
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Determine encoding from Content-Type
	contentType := req.Header.Get("Content-Type")
	isJSON := strings.Contains(contentType, "application/json")

	// Parse ExportTraceServiceRequest
	traceReq := &collectortrace.ExportTraceServiceRequest{}
	if isJSON {
		if err := protojson.Unmarshal(bodyBytes, traceReq); err != nil {
			r.log.Warn("failed to unmarshal JSON OTLP trace request", zap.Error(err))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		if err := proto.Unmarshal(bodyBytes, traceReq); err != nil {
			r.log.Warn("failed to unmarshal binary OTLP trace request", zap.Error(err))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Convert OTLP spans to ArgusSignals and enqueue
	var acceptedCount int32
	var rejectedCount int32

	for _, resourceSpan := range traceReq.GetResourceSpans() {
		resource := resourceSpan.GetResource()

		for _, scopeSpan := range resourceSpan.GetScopeSpans() {
			for _, span := range scopeSpan.GetSpans() {
				signal := r.mapSpanToSignal(span, resource)
				if signal == nil {
					rejectedCount++
					continue
				}

				err := r.queue.Enqueue(signal)
				if err != nil {
					if err == ErrQueueFull {
						r.log.Warn("queue full, OTLP span rejected", zap.String("span_id", hex.EncodeToString(span.SpanId)))
						rejectedCount++
					} else {
						r.log.Error("enqueue error", zap.String("span_id", hex.EncodeToString(span.SpanId)), zap.Error(err))
						rejectedCount++
					}
				} else {
					acceptedCount++
					if r.metrics != nil && r.metrics.SignalsReceived != nil {
						r.metrics.SignalsReceived.WithLabelValues("otlp").Inc()
					}
				}
			}
		}
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if r.metrics != nil && r.metrics.RequestDuration != nil {
		status := "ok"
		if rejectedCount > 0 {
			status = "partial"
		}
		r.metrics.RequestDuration.WithLabelValues("otlp", status).Observe(duration)
	}

	r.log.Debug("OTLP trace ingestion",
		zap.Int32("accepted", acceptedCount),
		zap.Int32("rejected", rejectedCount),
		zap.Float64("duration_seconds", duration),
	)

	// Send response
	resp := &collectortrace.ExportTraceServiceResponse{
		PartialSuccess: &collectortrace.ExportTracePartialSuccess{
			RejectedSpans: int64(rejectedCount),
		},
	}

	respBytes, err := proto.Marshal(resp)
	if err != nil {
		r.log.Error("failed to marshal response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

// handleMetrics handles POST /v1/metrics (OTLP ExportMetricsServiceRequest).
func (r *OTLPReceiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()

	// Read request body
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		r.log.Error("failed to read request body", zap.Error(err))
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Determine encoding from Content-Type
	contentType := req.Header.Get("Content-Type")
	isJSON := strings.Contains(contentType, "application/json")

	// Parse ExportMetricsServiceRequest
	// NOTE: OTLP metrics handling is simplified for Phase 2
	// Full implementation would map metric types to layer-specific contexts
	metricsReq := &collectormetrics.ExportMetricsServiceRequest{}
	if isJSON {
		if err := protojson.Unmarshal(bodyBytes, metricsReq); err != nil {
			r.log.Warn("failed to unmarshal JSON OTLP metrics request", zap.Error(err))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		if err := proto.Unmarshal(bodyBytes, metricsReq); err != nil {
			r.log.Warn("failed to unmarshal binary OTLP metrics request", zap.Error(err))
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// For now, accept metrics but don't enqueue them
	// Phase 2 treats metrics as acknowledged but not stored
	// Phase 3 will implement full metric-to-signal mapping

	var acceptedCount int32 = 0
	for _, resourceMetric := range metricsReq.GetResourceMetrics() {
		for _, scopeMetric := range resourceMetric.GetScopeMetrics() {
			acceptedCount += int32(len(scopeMetric.GetMetrics()))
		}
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if r.metrics != nil && r.metrics.RequestDuration != nil {
		r.metrics.RequestDuration.WithLabelValues("otlp", "ok").Observe(duration)
	}

	r.log.Debug("OTLP metrics ingestion",
		zap.Int32("accepted", acceptedCount),
		zap.Float64("duration_seconds", duration),
	)

	// Send response
	resp := &collectormetrics.ExportMetricsServiceResponse{
		PartialSuccess: &collectormetrics.ExportMetricsPartialSuccess{},
	}

	respBytes, err := proto.Marshal(resp)
	if err != nil {
		r.log.Error("failed to marshal response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

// mapSpanToSignal converts an OTLP span to an ArgusSignal.
// Returns nil if the span cannot be mapped (e.g., missing trace_id).
func (r *OTLPReceiver) mapSpanToSignal(span *tracepb.Span, resource *resourcepb.Resource) *v1.ArgusSignal {
	if span == nil || resource == nil {
		return nil
	}

	// Extract trace_id (convert from []byte to hex string)
	if len(span.TraceId) == 0 {
		r.log.Debug("span missing trace_id")
		return nil
	}
	traceID := hex.EncodeToString(span.TraceId)

	// Extract resource attributes
	resourceAttrs := make(map[string]string)
	if resource.GetAttributes() != nil {
		for _, attr := range resource.GetAttributes() {
			if attr.GetKey() != "" && attr.GetValue() != nil {
				switch val := attr.GetValue().Value.(type) {
				case *commonpb.AnyValue_StringValue:
					resourceAttrs[attr.GetKey()] = val.StringValue
				}
			}
		}
	}

	// Extract span attributes
	spanAttrs := make(map[string]string)
	if span.GetAttributes() != nil {
		for _, attr := range span.GetAttributes() {
			if attr.GetKey() != "" && attr.GetValue() != nil {
				switch val := attr.GetValue().Value.(type) {
				case *commonpb.AnyValue_StringValue:
					spanAttrs[attr.GetKey()] = val.StringValue
				}
			}
		}
	}

	// Determine app_id from resource attributes
	appID := resourceAttrs["service.name"]
	if appID == "" {
		appID = "unknown"
	}

	// Determine layer from span attributes or span name
	layer := r.inferLayer(span.GetName(), spanAttrs)

	// Determine severity from span status
	severity := r.inferSeverity(span.GetStatus(), spanAttrs)

	// Create ArgusSignal
	signal := &v1.ArgusSignal{
		SignalId:  fmt.Sprintf("otlp-%s", hex.EncodeToString(span.SpanId)),
		TraceId:   traceID,
		SpanId:    hex.EncodeToString(span.SpanId),
		Layer:     layer,
		Category:  span.GetName(),
		Severity:  severity,
		Source: &v1.Source{
			AppId:      appID,
			AppVersion: resourceAttrs["service.version"],
		},
		Timestamp: timestamppb.New(spanStartTimeToTime(span.GetStartTimeUnixNano())),
		IngestedAt: timestamppb.Now(),
	}

	// Set parent_span_id if present
	if len(span.GetParentSpanId()) > 0 {
		parentSpanID := hex.EncodeToString(span.GetParentSpanId())
		signal.ParentSpanId = &parentSpanID
	}

	// Calculate duration
	if span.GetEndTimeUnixNano() > 0 && span.GetStartTimeUnixNano() > 0 {
		durationNs := span.GetEndTimeUnixNano() - span.GetStartTimeUnixNano()
		durationMs := float32(durationNs) / 1_000_000.0
		signal.DurationMs = &durationMs
	}

	return signal
}

// inferLayer determines the Argus layer from span attributes or span name.
func (r *OTLPReceiver) inferLayer(spanName string, spanAttrs map[string]string) v1.Layer {
	// First, check for explicit argus.layer attribute
	if layerStr, ok := spanAttrs["argus.layer"]; ok {
		// Try to parse the layer string
		if layer, err := parseLayer(layerStr); err == nil {
			return layer
		}
	}

	// Infer from span name
	lowerName := strings.ToLower(spanName)
	if strings.HasPrefix(lowerName, "llm.") || strings.Contains(lowerName, "completion") {
		return v1.Layer_L5_OUTPUT_DECODING
	}
	if strings.HasPrefix(lowerName, "retrieval.") || strings.HasPrefix(lowerName, "rag.") {
		return v1.Layer_L7_RAG_RETRIEVAL
	}
	if strings.HasPrefix(lowerName, "agent.") || strings.HasPrefix(lowerName, "tool.") {
		return v1.Layer_L8_AGENTS
	}
	if strings.HasPrefix(lowerName, "safety.") {
		return v1.Layer_L6_SAFETY
	}

	// Default to application layer
	return v1.Layer_L10_APPLICATION
}

// inferSeverity determines the Argus severity from span status and attributes.
func (r *OTLPReceiver) inferSeverity(status *tracepb.Status, spanAttrs map[string]string) v1.Severity {
	// Check for explicit argus.severity attribute
	if severityStr, ok := spanAttrs["argus.severity"]; ok {
		if severity, err := parseSeverity(severityStr); err == nil {
			return severity
		}
	}

	// Check span status
	if status != nil && status.Code == tracepb.Status_STATUS_CODE_ERROR {
		return v1.Severity_HIGH
	}

	// Default to INFO
	return v1.Severity_INFO
}

// parseLayer parses a layer string to a v1.Layer enum.
func parseLayer(s string) (v1.Layer, error) {
	switch strings.ToUpper(s) {
	case "L1", "L1_HARDWARE":
		return v1.Layer_L1_HARDWARE, nil
	case "L2", "L2_MODEL_WEIGHTS":
		return v1.Layer_L2_MODEL_WEIGHTS, nil
	case "L3", "L3_TOKENIZER":
		return v1.Layer_L3_TOKENIZER, nil
	case "L4", "L4_TRANSFORMER":
		return v1.Layer_L4_TRANSFORMER, nil
	case "L5", "L5_OUTPUT_DECODING":
		return v1.Layer_L5_OUTPUT_DECODING, nil
	case "L6", "L6_SAFETY":
		return v1.Layer_L6_SAFETY, nil
	case "L7", "L7_RAG_RETRIEVAL":
		return v1.Layer_L7_RAG_RETRIEVAL, nil
	case "L8", "L8_AGENTS":
		return v1.Layer_L8_AGENTS, nil
	case "L9", "L9_API_GATEWAY":
		return v1.Layer_L9_API_GATEWAY, nil
	case "L10", "L10_APPLICATION":
		return v1.Layer_L10_APPLICATION, nil
	default:
		return v1.Layer_LAYER_UNSPECIFIED, fmt.Errorf("unknown layer: %s", s)
	}
}

// parseSeverity parses a severity string to a v1.Severity enum.
func parseSeverity(s string) (v1.Severity, error) {
	switch strings.ToUpper(s) {
	case "INFO":
		return v1.Severity_INFO, nil
	case "LOW":
		return v1.Severity_LOW, nil
	case "MEDIUM":
		return v1.Severity_MEDIUM, nil
	case "HIGH":
		return v1.Severity_HIGH, nil
	case "CRITICAL":
		return v1.Severity_CRITICAL, nil
	default:
		return v1.Severity_SEVERITY_UNSPECIFIED, fmt.Errorf("unknown severity: %s", s)
	}
}

// spanStartTimeToTime converts OTLP nanosecond timestamp to time.Time.
func spanStartTimeToTime(unixNano uint64) time.Time {
	return time.Unix(0, int64(unixNano))
}

// Note: We use go.opentelemetry.io/proto/otlp which provides the proto definitions.
// These types are imported from the OTLP proto package:
//   - collectortrace.ExportTraceServiceRequest
//   - collectortrace.ExportTraceServiceResponse
//   - collectormetrics.ExportMetricsServiceRequest
//   - collectormetrics.ExportMetricsServiceResponse
//   - commonpb.Resource
//   - tracepb.Span
//   - tracepb.Status
