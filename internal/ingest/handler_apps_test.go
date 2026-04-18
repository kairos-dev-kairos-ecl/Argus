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

// TestListAppsNilPool verifies handleListApps returns 503 when the PostgreSQL
// pool is nil (graceful degradation, not a stub).
func TestListAppsNilPool(t *testing.T) {
	h := &QueryHandler{pool: nil, log: zap.NewNop()}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	rec := httptest.NewRecorder()
	h.handleListApps(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "storage unavailable", body["error"])
}

// TestCreateAppNilPool verifies handleCreateApp returns 503 when the pool is
// nil — the nil-pool guard must fire BEFORE any JSON parsing or validation.
func TestCreateAppNilPool(t *testing.T) {
	h := &QueryHandler{pool: nil, log: zap.NewNop()}

	body := strings.NewReader(`{"name":"test-app","description":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", body)
	rec := httptest.NewRecorder()
	h.handleCreateApp(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "storage unavailable", resp["error"])
}

// TestGetAppKeyNilPool verifies handleGetAppKey returns 503 when the pool is nil.
func TestGetAppKeyNilPool(t *testing.T) {
	h := &QueryHandler{pool: nil, log: zap.NewNop()}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/apps/some-id/key", nil)
	rec := httptest.NewRecorder()
	h.handleGetAppKey(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "storage unavailable", resp["error"])
}

// TestRotateAppKeyNilPool verifies handleRotateAppKey returns 503 when the pool is nil.
func TestRotateAppKeyNilPool(t *testing.T) {
	h := &QueryHandler{pool: nil, log: zap.NewNop()}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/some-id/key/rotate", nil)
	rec := httptest.NewRecorder()
	h.handleRotateAppKey(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "storage unavailable", resp["error"])
}
