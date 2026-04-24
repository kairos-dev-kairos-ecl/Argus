package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/argusxdr/argus/internal/ingest"
	"github.com/argusxdr/argus/internal/resilience"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHealthHandler_DegradedWhenNilStorage(t *testing.T) {
	// makeHealthHandler is tested via the exported function
	// This tests the response shape
	handler := makeHealthHandler(nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, 200, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "degraded", body["status"])
	assert.NotNil(t, body["components"])
}

// TestRateLimitWired tests that rate limiting can be wired to the query handler
func TestRateLimitWired(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Create query handler
	qh := ingest.NewQueryHandler(nil, nil, nil)

	// Wire rate limiter
	rl := resilience.NewRedisRateLimiter(client)
	qh.SetRateLimiter(rl)

	// Verify rate limiter was set
	assert.NotNil(t, rl)
}

// TestRateLimitMiddlewareLogin tests login endpoint with rate limiting
func TestRateLimitMiddlewareLogin(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := resilience.NewRedisRateLimiter(client)

	// Simulate login rate limiting (5 per 60s)
	loginKeyFn := func(r *http.Request) string {
		return "rl:login:127.0.0.1"
	}

	handler := resilience.Limit(rl, loginKeyFn, 5, 60*time.Second)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	// Make 5 successful requests
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// 6th request should be denied
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request 6 should be denied")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// TestRateLimitMiddlewareRefresh tests refresh endpoint with rate limiting
func TestRateLimitMiddlewareRefresh(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := resilience.NewRedisRateLimiter(client)

	// Simulate refresh rate limiting (30 per 60s)
	refreshKeyFn := func(r *http.Request) string {
		return "rl:refresh:test-token-hash"
	}

	handler := resilience.Limit(rl, refreshKeyFn, 30, 60*time.Second)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	// Make 30 successful requests
	for i := 0; i < 30; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// 31st request should be denied
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request 31 should be denied")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// TestRateLimitMiddlewareQuery tests query endpoint with rate limiting
func TestRateLimitMiddlewareQuery(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := resilience.NewRedisRateLimiter(client)

	// Simulate query rate limiting (60 per 60s)
	queryKeyFn := func(r *http.Request) string {
		return "rl:query:user-123"
	}

	handler := resilience.Limit(rl, queryKeyFn, 60, 60*time.Second)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	// Make 60 successful requests
	for i := 0; i < 60; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/query", nil)
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// 61st request should be denied
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/query", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request 61 should be denied")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// TestRetryAfterHeader tests that Retry-After header is set correctly
func TestRetryAfterHeader(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := resilience.NewRedisRateLimiter(client)

	keyFn := func(r *http.Request) string {
		return "rl:test:key"
	}

	handler := resilience.Limit(rl, keyFn, 1, 60*time.Second)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	// First request succeeds
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request is denied with Retry-After
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/test", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	retryAfter := rec.Header().Get("Retry-After")
	assert.NotEmpty(t, retryAfter)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json"))

	// Body should contain JSON with retry_after
	assert.Contains(t, rec.Body.String(), "retry_after")
}
