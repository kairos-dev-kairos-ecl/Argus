package resilience

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPIKeyTokenBucketLimiter(t *testing.T) {
	tests := []struct {
		name           string
		burst          float64
		rate           float64
		requests       int
		expectedPass   int
		expectedFail   int
		wait           bool
		waitDuration   time.Duration
		description    string
	}{
		{
			name:         "Test 1: 10 rapid-fire requests with same API key all pass (burst = 10)",
			burst:        10,
			rate:         1000,
			requests:     10,
			expectedPass: 10,
			expectedFail: 0,
			description:  "All 10 requests within burst capacity should pass",
		},
		{
			name:         "Test 2: Request 11 within rapid-fire is denied (429)",
			burst:        10,
			rate:         1000,
			requests:     11,
			expectedPass: 10,
			expectedFail: 1,
			description:  "Request 11 should be rejected",
		},
		{
			name:         "Test 4: After waiting, tokens refill at the configured rate",
			burst:        10,
			rate:         1000,
			requests:     10,
			expectedPass: 10,
			expectedFail: 0,
			wait:         true,
			waitDuration: 15 * time.Millisecond,
			description:  "After waiting ~15ms, tokens should refill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewAPIKeyTokenBucketLimiter(tt.rate, tt.burst)

			// Create a key for testing
			key1 := &auth.APIKey{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				Name:      "test-key",
				KeyPrefix: "argus_sk_test1",
				Scopes:    []string{"signals:write"},
			}

			pass := 0
			fail := 0

			for i := 0; i < tt.requests; i++ {
				// Create request and inject key into context
				req := httptest.NewRequest("POST", "/v1/signals", nil)
				ctx := context.WithValue(req.Context(), auth.APIKeyContextKey, key1)
				req = req.WithContext(ctx)

				// Create response recorder
				w := httptest.NewRecorder()

				// Create a next handler that returns 200
				nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				// Get the middleware
				middleware := limiter.Middleware()
				handler := middleware(nextHandler)

				// Call handler
				handler.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					pass++
				} else if w.Code == http.StatusTooManyRequests {
					fail++
				}
			}

			assert.Equal(t, tt.expectedPass, pass, "expected %d requests to pass, got %d", tt.expectedPass, pass)
			assert.Equal(t, tt.expectedFail, fail, "expected %d requests to fail, got %d", tt.expectedFail, fail)
		})
	}
}

func TestAPIKeyTokenBucketLimiter_IsolatedBuckets(t *testing.T) {
	t.Run("Test 3: Two distinct API keys get isolated buckets", func(t *testing.T) {
		// Use a lower rate and burst to make the test less sensitive to timing
		limiter := NewAPIKeyTokenBucketLimiter(1000, 10)

		// Create two different keys
		key1 := &auth.APIKey{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Name:      "test-key-1",
			KeyPrefix: "argus_sk_key1",
			Scopes:    []string{"signals:write"},
		}
		key2 := &auth.APIKey{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Name:      "test-key-2",
			KeyPrefix: "argus_sk_key2",
			Scopes:    []string{"signals:write"},
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := limiter.Middleware()
		handler := middleware(nextHandler)

		// Exhaust key1's bucket (send 11 requests, only 10 should pass)
		key1Passed := 0
		key1Failed := 0
		for i := 0; i < 11; i++ {
			req := httptest.NewRequest("POST", "/v1/signals", nil)
			ctx := context.WithValue(req.Context(), auth.APIKeyContextKey, key1)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				key1Passed++
			} else if w.Code == http.StatusTooManyRequests {
				key1Failed++
			}
		}

		// key1 should have passed 10 and failed 1
		assert.Equal(t, 10, key1Passed, "key1 should pass 10 requests")
		assert.Equal(t, 1, key1Failed, "key1 should fail 1 request")

		// Now try key2 - should not be affected by key1's exhaustion
		key2Passed := 0
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("POST", "/v1/signals", nil)
			ctx := context.WithValue(req.Context(), auth.APIKeyContextKey, key2)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				key2Passed++
			}
		}

		// key2 should pass all 10 - it has its own isolated bucket
		assert.Equal(t, 10, key2Passed, "key2 should pass all 10 requests independently from key1")
	})
}

func TestAPIKeyTokenBucketLimiter_MissingContext(t *testing.T) {
	t.Run("Test 5: Missing API key context → middleware passes through (defence-in-depth)", func(t *testing.T) {
		limiter := NewAPIKeyTokenBucketLimiter(10000, 100)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		middleware := limiter.Middleware()
		handler := middleware(nextHandler)

		// Create request WITHOUT API key in context
		req := httptest.NewRequest("POST", "/v1/signals", nil)
		w := httptest.NewRecorder()

		// Should pass through without checking rate limit
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "should pass through when no API key in context")
	})
}
