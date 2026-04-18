package ingest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLayerStatusResponseShape verifies the GET /api/v1/layers/status response
// serializes with the exact field names the frontend expects.
func TestLayerStatusResponseShape(t *testing.T) {
	h := &QueryHandler{
		ch:  nil, // degraded mode — returns all-gray static response
		log: zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/layers/status", nil)
	rec := httptest.NewRecorder()
	h.handleGetLayerStatus(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	layers, ok := resp["layers"]
	require.True(t, ok, "response must have 'layers' key")

	layersSlice, ok := layers.([]interface{})
	require.True(t, ok, "'layers' must be an array")
	assert.Len(t, layersSlice, 10, "must have exactly 10 layers (L1–L10)")

	first, ok := layersSlice[0].(map[string]interface{})
	require.True(t, ok, "each layer entry must be an object")

	// layer name must be string enum, not an integer
	layerVal, ok := first["layer"]
	require.True(t, ok, "entry must have 'layer' key")
	assert.Equal(t, "L1_HARDWARE", layerVal, "first layer must be L1_HARDWARE string")

	// status must be "gray" in degraded mode (not "unknown" or "idle")
	statusVal, ok := first["status"]
	require.True(t, ok, "entry must have 'status' key")
	assert.Equal(t, "gray", statusVal)

	// signal_count_5min key (not signal_count)
	_, hasSignalCount5min := first["signal_count_5min"]
	assert.True(t, hasSignalCount5min, "must have 'signal_count_5min' key, not 'signal_count'")

	// last_signal_time key (not last_seen_at)
	_, hasLastSignalTime := first["last_signal_time"]
	assert.True(t, hasLastSignalTime, "must have 'last_signal_time' key, not 'last_seen_at'")

	// no "name" key
	_, hasName := first["name"]
	assert.False(t, hasName, "must NOT have 'name' key")
}

// TestTraceResponseShape verifies the TraceResponse struct serializes with
// the correct field names (spans, not signals).
func TestTraceResponseShape(t *testing.T) {
	parentID := "parent-signal-123"
	tr := TraceResponse{
		TraceID: "trace-abc",
		Spans: []SpanView{
			{
				SignalID:       "sig-001",
				Layer:          "L3_TOKENIZER",
				StartTime:      "2024-01-01T00:00:00Z",
				DurationMs:     12.5,
				ParentSignalID: parentID,
				Status:         "ok",
				Message:        "tokenize",
			},
		},
		Detections: []Detection{},
		DurationMs: 42,
	}

	data, err := json.Marshal(tr)
	require.NoError(t, err)

	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &obj))

	// spans key (NOT signals)
	_, hasSpans := obj["spans"]
	assert.True(t, hasSpans, "TraceResponse must have 'spans' key, not 'signals'")
	_, hasSignals := obj["signals"]
	assert.False(t, hasSignals, "TraceResponse must NOT have 'signals' key")

	// detections key
	_, hasDetections := obj["detections"]
	assert.True(t, hasDetections, "TraceResponse must have 'detections' key")

	// duration_ms key
	_, hasDuration := obj["duration_ms"]
	assert.True(t, hasDuration, "TraceResponse must have 'duration_ms' key")

	// check SpanView field names
	spansArr, ok := obj["spans"].([]interface{})
	require.True(t, ok)
	require.Len(t, spansArr, 1)

	span, ok := spansArr[0].(map[string]interface{})
	require.True(t, ok)

	for _, key := range []string{"start_time", "parent_signal_id", "status", "message"} {
		_, exists := span[key]
		assert.True(t, exists, "SpanView must have %q key", key)
	}
}

// TestQueryResponseShape verifies QueryResultResponse serializes with rows as
// objects (not arrays) and the correct field names.
func TestQueryResponseShape(t *testing.T) {
	qr := QueryResultResponse{
		Rows: []map[string]interface{}{
			{"signal_id": "sig-1", "layer": 3},
			{"signal_id": "sig-2", "layer": 7},
		},
		Total:           2,
		ExecutionTimeMs: 15,
	}

	data, err := json.Marshal(qr)
	require.NoError(t, err)

	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &obj))

	// execution_time_ms key
	_, hasExecMs := obj["execution_time_ms"]
	assert.True(t, hasExecMs, "must have 'execution_time_ms' key")

	// total key
	_, hasTotal := obj["total"]
	assert.True(t, hasTotal, "must have 'total' key")

	// rows must be array of objects (not array of arrays)
	rowsVal, ok := obj["rows"]
	require.True(t, ok, "must have 'rows' key")
	rowsArr, ok := rowsVal.([]interface{})
	require.True(t, ok, "'rows' must be an array")
	require.Len(t, rowsArr, 2)

	firstRow, ok := rowsArr[0].(map[string]interface{})
	require.True(t, ok, "each row must be an object (not an array)")
	_, hasSignalID := firstRow["signal_id"]
	assert.True(t, hasSignalID)

	// no "columns" or "row_count" keys
	_, hasColumns := obj["columns"]
	assert.False(t, hasColumns, "must NOT have 'columns' key")
	_, hasRowCount := obj["row_count"]
	assert.False(t, hasRowCount, "must NOT have 'row_count' key")
}

// TestRulesNilPool proves that the rules handlers are DB-backed (not 501 stubs).
// With nil pool they must return 503 with "database unavailable", not 501.
func TestRulesNilPool(t *testing.T) {
	h := &QueryHandler{
		pool: nil, // simulate PostgreSQL unavailable
		log:  zap.NewNop(),
	}

	t.Run("GET /api/v1/rules returns 503 with nil pool", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
		rec := httptest.NewRecorder()
		h.handleListRules(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var body map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "database unavailable", body["error"])
	})

	t.Run("POST /api/v1/rules returns 503 with nil pool (nil-pool guard runs before JSON parse)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rules", strings.NewReader("{}"))
		rec := httptest.NewRecorder()
		h.handleCreateRule(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var body map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "database unavailable", body["error"])
	})
}
