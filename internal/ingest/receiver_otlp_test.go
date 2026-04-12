package ingest

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestOTLPReceiverMapSpanWithLayerAttribute(t *testing.T) {
	receiver := NewOTLPReceiver(nil, nil, zap.NewNop())

	// Create OTLP span with argus.layer attribute
	span := &tracepb.Span{
		TraceId:              []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanId:               []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		Name:                 "test.span",
		StartTimeUnixNano:    1234567890000000000,
		EndTimeUnixNano:      1234567890000100000,
		Attributes: []*commonpb.KeyValue{
			{
				Key: "argus.layer",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{
						StringValue: "L7_RAG_RETRIEVAL",
					},
				},
			},
		},
	}

	resource := &commonpb.Resource{
		Attributes: []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{
						StringValue: "test-app",
					},
				},
			},
		},
	}

	signal := receiver.mapSpanToSignal(span, resource)

	assert.NotNil(t, signal)
	assert.Equal(t, "test-app", signal.Source.AppId)
	assert.Equal(t, v1.Layer_L7_RAG_RETRIEVAL, signal.Layer)
	assert.Equal(t, "test.span", signal.Category)
}

func TestOTLPReceiverMapSpanLayerInference(t *testing.T) {
	receiver := NewOTLPReceiver(nil, nil, zap.NewNop())

	testCases := []struct {
		name         string
		spanName     string
		expectedLayer v1.Layer
	}{
		{
			name:         "llm completion",
			spanName:     "llm.completion",
			expectedLayer: v1.Layer_L5_OUTPUT_DECODING,
		},
		{
			name:         "rag retrieval",
			spanName:     "rag.vector_search",
			expectedLayer: v1.Layer_L7_RAG_RETRIEVAL,
		},
		{
			name:         "retrieval search",
			spanName:     "retrieval.search",
			expectedLayer: v1.Layer_L7_RAG_RETRIEVAL,
		},
		{
			name:         "agent tool call",
			spanName:     "agent.tool_call",
			expectedLayer: v1.Layer_L8_AGENTS,
		},
		{
			name:         "safety classifier",
			spanName:     "safety.classifier",
			expectedLayer: v1.Layer_L6_SAFETY,
		},
		{
			name:         "default application",
			spanName:     "app.request",
			expectedLayer: v1.Layer_L10_APPLICATION,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			span := &tracepb.Span{
				TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
				SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
				Name:              tc.spanName,
				StartTimeUnixNano: 1234567890000000000,
			}

			resource := &commonpb.Resource{
				Attributes: []*commonpb.KeyValue{
					{
						Key: "service.name",
						Value: &commonpb.AnyValue{
							Value: &commonpb.AnyValue_StringValue{
								StringValue: "test-app",
							},
						},
					},
				},
			}

			signal := receiver.mapSpanToSignal(span, resource)
			assert.NotNil(t, signal)
			assert.Equal(t, tc.expectedLayer, signal.Layer)
		})
	}
}

func TestOTLPReceiverMapSpanErrorSeverity(t *testing.T) {
	receiver := NewOTLPReceiver(nil, nil, zap.NewNop())

	span := &tracepb.Span{
		TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		Name:              "test.span",
		StartTimeUnixNano: 1234567890000000000,
		Status: &tracepb.Status{
			Code: tracepb.Status_STATUS_CODE_ERROR,
		},
	}

	resource := &commonpb.Resource{
		Attributes: []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{
						StringValue: "test-app",
					},
				},
			},
		},
	}

	signal := receiver.mapSpanToSignal(span, resource)

	assert.NotNil(t, signal)
	assert.Equal(t, v1.Severity_HIGH, signal.Severity)
}

func TestOTLPReceiverMapSpanDuration(t *testing.T) {
	receiver := NewOTLPReceiver(nil, nil, zap.NewNop())

	span := &tracepb.Span{
		TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		Name:              "test.span",
		StartTimeUnixNano: 1000000000,   // 1 second
		EndTimeUnixNano:   1100000000,   // 1.1 seconds = 100ms duration
	}

	resource := &commonpb.Resource{
		Attributes: []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{
						StringValue: "test-app",
					},
				},
			},
		},
	}

	signal := receiver.mapSpanToSignal(span, resource)

	assert.NotNil(t, signal)
	assert.NotNil(t, signal.DurationMs)
	assert.InDelta(t, 100.0, *signal.DurationMs, 0.1)
}

func TestOTLPReceiverHandleTracesBinary(t *testing.T) {
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	metricsReg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(metricsReg)
	receiver := NewOTLPReceiver(queue, ingestMetrics, log)

	// Create OTLP trace request with binary protobuf
	traceReq := &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &commonpb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test-app",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
								SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
								Name:              "test.span",
								StartTimeUnixNano: 1234567890000000000,
								EndTimeUnixNano:   1234567890000100000,
							},
						},
					},
				},
			},
		},
	}

	reqBytes, err := proto.Marshal(traceReq)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.handleTraces(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-protobuf", rec.Header().Get("Content-Type"))

	// Verify signal is in queue
	assert.Equal(t, 1, queue.Depth())
}

func TestOTLPReceiverHandleTracesJSON(t *testing.T) {
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	metricsReg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(metricsReg)
	receiver := NewOTLPReceiver(queue, ingestMetrics, log)

	// Create OTLP trace request (protojson will handle JSON marshaling)
	traceReq := &collectortrace.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &commonpb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test-app",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
								SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
								Name:              "test.span",
								StartTimeUnixNano: 1234567890000000000,
								EndTimeUnixNano:   1234567890000100000,
							},
						},
					},
				},
			},
		},
	}

	// Use protojson for JSON marshaling
	reqJSON, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(traceReq)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.handleTraces(rec, req)

	// Verify response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-protobuf", rec.Header().Get("Content-Type"))

	// Verify signal is in queue
	assert.Equal(t, 1, queue.Depth())
}

func TestOTLPReceiverHandleTracesMalformedBinary(t *testing.T) {
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	receiver := NewOTLPReceiver(queue, nil, log)

	// Create malformed binary data
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader([]byte{0xFF, 0xFE, 0xFD}))
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.handleTraces(rec, req)

	// Verify response is 400
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Verify queue is empty
	assert.Equal(t, 0, queue.Depth())
}

func TestOTLPReceiverMapSpanMissingTraceID(t *testing.T) {
	receiver := NewOTLPReceiver(nil, nil, zap.NewNop())

	span := &tracepb.Span{
		// Missing TraceId
		SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		Name:              "test.span",
		StartTimeUnixNano: 1234567890000000000,
	}

	resource := &commonpb.Resource{}

	signal := receiver.mapSpanToSignal(span, resource)

	// Should return nil when trace_id is missing
	assert.Nil(t, signal)
}

func TestOTLPReceiverParseLayer(t *testing.T) {
	testCases := []struct {
		input    string
		expected v1.Layer
		hasError bool
	}{
		{"L1_HARDWARE", v1.Layer_L1_HARDWARE, false},
		{"L5_OUTPUT_DECODING", v1.Layer_L5_OUTPUT_DECODING, false},
		{"L7_RAG_RETRIEVAL", v1.Layer_L7_RAG_RETRIEVAL, false},
		{"L10", v1.Layer_L10_APPLICATION, false},
		{"l8", v1.Layer_L8_AGENTS, false},
		{"INVALID", v1.Layer_LAYER_UNSPECIFIED, true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			layer, err := parseLayer(tc.input)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, layer)
			}
		})
	}
}

func TestOTLPReceiverParseSeverity(t *testing.T) {
	testCases := []struct {
		input    string
		expected v1.Severity
		hasError bool
	}{
		{"INFO", v1.Severity_INFO, false},
		{"LOW", v1.Severity_LOW, false},
		{"MEDIUM", v1.Severity_MEDIUM, false},
		{"HIGH", v1.Severity_HIGH, false},
		{"CRITICAL", v1.Severity_CRITICAL, false},
		{"info", v1.Severity_INFO, false},
		{"INVALID", v1.Severity_SEVERITY_UNSPECIFIED, true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			severity, err := parseSeverity(tc.input)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, severity)
			}
		})
	}
}

