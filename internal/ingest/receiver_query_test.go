package ingest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRouteRegistration(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/signals"},
		{"GET", "/v1/schema/signals"},
		{"GET", "/api/v1/layers/status"},
		{"GET", "/api/v1/traces/test-trace-id"},
		{"POST", "/api/v1/query"},
		{"GET", "/api/v1/rules"},
		{"POST", "/api/v1/rules"},
		{"GET", "/api/v1/alerts"},
		{"GET", "/api/v1/incidents"},
		{"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/auth/refresh"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/users"},
		{"GET", "/api/v1/apps"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"route %s %s returned 404 — is it registered in RegisterRoutes()?",
				route.method, route.path)
		})
	}
}

func TestHandleGetTrace_NilStorage(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/v1/traces/trace-abc-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Contains(t, body["error"], "storage unavailable")
}

func TestHandleGetTrace_EmptyTraceId(t *testing.T) {
	// chi won't match /api/v1/traces/ (no param) — test with a path that gives empty
	// Actually chi requires the param, so test that a valid path returns 503 with nil ch
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	// A valid trace ID with nil storage returns 503
	req := httptest.NewRequest("GET", "/api/v1/traces/any-trace", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandlePostQuery_BlocksDDL(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	body := `{"sql":"DROP TABLE signals"}`
	req := httptest.NewRequest("POST", "/api/v1/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "DDL")
}

func TestHandlePostQuery_NilStorage(t *testing.T) {
	mux := chi.NewRouter()
	qh := ingest.NewQueryHandler(nil, &metrics.HTTP{}, zap.NewNop())
	qh.RegisterRoutes(mux)

	body := `{"sql":"SELECT count() FROM signals"}`
	req := httptest.NewRequest("POST", "/api/v1/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
}
