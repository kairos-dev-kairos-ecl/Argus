package ingest_test

import (
	"net/http"
	"net/http/httptest"
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
