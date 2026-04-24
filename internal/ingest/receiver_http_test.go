package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHTTPReceiverPostSignalsSingle(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	metricsReg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(metricsReg)
	receiver := NewHTTPReceiver(queue, testAuth, ingestMetrics, nil, log)

	// Create a single signal
	signal := &v1.ArgusSignal{
		SignalId:  "test-signal-001",
		TraceId:   "trace-001",
		Layer:     v1.Layer_L10_APPLICATION,
		Category:  "test.signal",
		Severity:  v1.Severity_INFO,
		Source:    &v1.Source{AppId: "test-app"},
		Timestamp: timestamppb.Now(),
	}

	// Marshal to JSON
	signalJSON, err := protojson.Marshal(signal)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(signalJSON))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(1), resp["accepted"])
	assert.Equal(t, float64(0), resp["rejected"])

	// Verify signal is in queue
	assert.Equal(t, 1, queue.Depth())
}

func TestHTTPReceiverPostSignalsArray(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	metricsReg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(metricsReg)
	receiver := NewHTTPReceiver(queue, testAuth, ingestMetrics, nil, log)

	// Create multiple signals
	signals := []*v1.ArgusSignal{
		{
			SignalId:  "test-signal-001",
			TraceId:   "trace-001",
			Layer:     v1.Layer_L10_APPLICATION,
			Category:  "test.signal",
			Severity:  v1.Severity_INFO,
			Source:    &v1.Source{AppId: "test-app"},
			Timestamp: timestamppb.Now(),
		},
		{
			SignalId:  "test-signal-002",
			TraceId:   "trace-001",
			Layer:     v1.Layer_L7_RAG_RETRIEVAL,
			Category:  "retrieval.search",
			Severity:  v1.Severity_INFO,
			Source:    &v1.Source{AppId: "test-app"},
			Timestamp: timestamppb.Now(),
		},
		{
			SignalId:  "test-signal-003",
			TraceId:   "trace-001",
			Layer:     v1.Layer_L5_OUTPUT_DECODING,
			Category:  "output.generation",
			Severity:  v1.Severity_INFO,
			Source:    &v1.Source{AppId: "test-app"},
			Timestamp: timestamppb.Now(),
		},
	}

	// Marshal to JSON array
	var signalsJSON []json.RawMessage
	for _, sig := range signals {
		sigJSON, err := protojson.Marshal(sig)
		require.NoError(t, err)
		signalsJSON = append(signalsJSON, sigJSON)
	}
	arrayJSON, err := json.Marshal(signalsJSON)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(arrayJSON))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(3), resp["accepted"])
	assert.Equal(t, float64(0), resp["rejected"])

	// Verify all signals are in queue
	assert.Equal(t, 3, queue.Depth())
}

func TestHTTPReceiverMissingAuth(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Create request WITHOUT auth header
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader([]byte(`{"signal_id":"test"}`)))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler directly (bypasses route registration)
	// We need to wrap with middleware to test auth
	handler := testAuth.HTTPMiddleware()(http.HandlerFunc(receiver.HandlePostSignals))
	handler.ServeHTTP(rec, req)

	// Verify response is 401
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthenticated")
}

func TestHTTPReceiverMalformedJSON(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Create malformed JSON
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader([]byte(`{invalid json}`)))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 400
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}

func TestHTTPReceiverOversizedBody(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Create oversized body (>4MB)
	oversizedBody := make([]byte, 5*1024*1024) // 5MB
	for i := range oversizedBody {
		oversizedBody[i] = 'a'
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(oversizedBody))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 413 Payload Too Large
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), "request body too large")
}

func TestHTTPReceiverMissingRequiredFields(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Create signal without trace_id
	signal := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		// missing TraceId
		Layer:     v1.Layer_L10_APPLICATION,
		Category:  "test.signal",
		Severity:  v1.Severity_INFO,
		Timestamp: timestamppb.Now(),
	}

	// Marshal to JSON
	signalJSON, err := protojson.Marshal(signal)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(signalJSON))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 400
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing required fields")
}

func TestHTTPReceiverQueueFull(t *testing.T) {
	// Setup with very small queue
	queue := NewQueue(1, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Enqueue a signal to fill the queue
	queue.Enqueue(&v1.ArgusSignal{
		SignalId: "fill-queue",
		TraceId:  "trace-fill",
	})

	// Create another signal
	signal := &v1.ArgusSignal{
		SignalId:  "test-signal-001",
		TraceId:   "trace-001",
		Layer:     v1.Layer_L10_APPLICATION,
		Category:  "test.signal",
		Severity:  v1.Severity_INFO,
		Timestamp: timestamppb.Now(),
	}

	// Marshal to JSON
	signalJSON, err := protojson.Marshal(signal)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(signalJSON))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 429 (all signals rejected)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "ingest queue full")
}

func TestHTTPReceiverPartialRejection(t *testing.T) {
	// Setup with small queue
	queue := NewQueue(2, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	metricsReg := prometheus.NewRegistry()
	ingestMetrics := metrics.NewIngest(metricsReg)
	receiver := NewHTTPReceiver(queue, testAuth, ingestMetrics, nil, log)

	// Enqueue one signal to fill the queue partially
	queue.Enqueue(&v1.ArgusSignal{
		SignalId: "fill-queue",
		TraceId:  "trace-fill",
	})

	// Create array of 3 signals (but queue only has 1 slot left)
	signals := []*v1.ArgusSignal{
		{
			SignalId:  "test-signal-001",
			TraceId:   "trace-001",
			Layer:     v1.Layer_L10_APPLICATION,
			Timestamp: timestamppb.Now(),
		},
		{
			SignalId:  "test-signal-002",
			TraceId:   "trace-002",
			Layer:     v1.Layer_L10_APPLICATION,
			Timestamp: timestamppb.Now(),
		},
		{
			SignalId:  "test-signal-003",
			TraceId:   "trace-003",
			Layer:     v1.Layer_L10_APPLICATION,
			Timestamp: timestamppb.Now(),
		},
	}

	// Marshal to JSON array
	var signalsJSON []json.RawMessage
	for _, sig := range signals {
		sigJSON, err := protojson.Marshal(sig)
		require.NoError(t, err)
		signalsJSON = append(signalsJSON, sigJSON)
	}
	arrayJSON, err := json.Marshal(signalsJSON)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(arrayJSON))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 207 (partial success)
	assert.Equal(t, http.StatusMultiStatus, rec.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(1), resp["accepted"])
	assert.Equal(t, float64(2), resp["rejected"])
	assert.NotNil(t, resp["errors"])
}

func TestHTTPReceiverEmptyBody(t *testing.T) {
	// Setup
	queue := NewQueue(1000, nil)
	log := zap.NewNop()
	testAuth := NewTestAuthValidator(log, map[string]string{
		"test-key": "test-app",
	})
	receiver := NewHTTPReceiver(queue, testAuth, nil, nil, log)

	// Create request with empty body
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader([]byte("")))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// Call handler
	receiver.HandlePostSignals(rec, req)

	// Verify response is 400
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "empty request body")
}
