package resilience

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisRateLimiterAllow tests Allow() returns true for first N requests within window
func TestRedisRateLimiterAllow(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	ctx := context.Background()
	key := "test:key"
	limit := 3
	window := 60 * time.Second

	// First 3 requests should be allowed
	for i := 0; i < limit; i++ {
		allowed, _, err := rl.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	// 4th request should be denied
	allowed, _, err := rl.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed, "request 4 should be denied")
}

// TestRedisRateLimiterRetryAfter tests retryAfterSec is > 0 and <= window when denied
func TestRedisRateLimiterRetryAfter(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	ctx := context.Background()
	key := "test:retry"
	limit := 2
	window := 60 * time.Second

	// Exhaust limit
	for i := 0; i < limit; i++ {
		rl.Allow(ctx, key, limit, window)
	}

	// Next request should be denied with retryAfter
	allowed, retryAfter, err := rl.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Greater(t, retryAfter, 0, "retryAfter should be positive")
	assert.LessOrEqual(t, retryAfter, int(window.Seconds())+1, "retryAfter should be <= window")
}

// TestRedisRateLimiterIsolation tests keys are isolated
func TestRedisRateLimiterIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	ctx := context.Background()
	limit := 2
	window := 60 * time.Second

	// Exhaust limit for key1
	for i := 0; i < limit; i++ {
		rl.Allow(ctx, "key:1", limit, window)
	}

	// key2 should not be affected
	allowed, _, err := rl.Allow(ctx, "key:2", limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "key2 should not be affected by key1 exhaustion")
}

// TestRedisRateLimiterReset tests limit resets after window elapses
func TestRedisRateLimiterReset(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	ctx := context.Background()
	key := "test:reset"
	limit := 2
	window := 1 * time.Second

	// Exhaust limit
	for i := 0; i < limit; i++ {
		rl.Allow(ctx, key, limit, window)
	}

	// Should be denied
	allowed, _, _ := rl.Allow(ctx, key, limit, window)
	assert.False(t, allowed)

	// Wait for window to elapse
	time.Sleep(window + 100*time.Millisecond)

	// Now should be allowed again
	allowed, _, err := rl.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	assert.True(t, allowed, "limit should reset after window elapses")
}

// TestRateLimitMiddlewareRetryAfterHeader tests middleware writes Retry-After on deny
func TestRateLimitMiddlewareRetryAfterHeader(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	requestCount := 0
	keyFn := func(r *http.Request) string {
		return "test:mw"
	}
	limit := 2
	window := 60 * time.Second

	handler := Limit(rl, keyFn, limit, window)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests
	for i := 0; i < limit; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		handler.ServeHTTP(rec, req)
		requestCount++
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// Next request should be denied with Retry-After header
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "request %d (3rd) should be denied", requestCount+1)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "Retry-After header should be set")
}

// TestRateLimitMiddlewarePassthrough tests middleware passes through on allow
func TestRateLimitMiddlewarePassthrough(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	keyFn := func(r *http.Request) string {
		return "test:passthrough"
	}
	limit := 5
	window := 60 * time.Second

	called := false
	handler := Limit(rl, keyFn, limit, window)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "handler should be called when request is allowed")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRateLimitMiddlewareFailOpen tests middleware fails open on Redis error
func TestRateLimitMiddlewareFailOpen(t *testing.T) {
	// Use a limiter with closed Redis
	client := redis.NewClient(&redis.Options{Addr: "invalid:9999"})

	rl := NewRedisRateLimiter(client)
	keyFn := func(r *http.Request) string {
		return "test:failopen"
	}
	limit := 2
	window := 60 * time.Second

	called := false
	handler := Limit(rl, keyFn, limit, window)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Should not panic and should fail open (allow the request)
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "handler should be called (fail-open)")
	assert.Equal(t, http.StatusOK, rec.Code)
	// Check for degradation header
	assert.Equal(t, "true", rec.Header().Get("X-RateLimit-Degraded"))
}

// TestRateLimitMiddlewareEmptyKey tests middleware handles empty key gracefully
func TestRateLimitMiddlewareEmptyKey(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	rl := NewRedisRateLimiter(client)
	keyFn := func(r *http.Request) string {
		return "" // Return empty key
	}
	limit := 2
	window := 60 * time.Second

	called := false
	handler := Limit(rl, keyFn, limit, window)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)

	assert.True(t, called, "handler should be called when key is empty (pass-through)")
	assert.Equal(t, http.StatusOK, rec.Code)
}
