package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHealthHandler_DegradedWhenNilStorage(t *testing.T) {
	// makeHealthHandler is tested via the exported function
	// This tests the response shape
	handler := makeHealthHandler(nil, zap.NewNop())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "degraded", body["status"])
	assert.NotNil(t, body["components"])
}
