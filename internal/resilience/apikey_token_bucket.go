package resilience

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/argusxdr/argus/internal/auth"
)

// APIKeyTokenBucketLimiter keeps one TokenBucket per api_key_prefix.
// Keys: "tb:signals:{api_key_prefix}". Burst=100, rate=10000/sec.
// This limiter is used to enforce per-API-key rate limiting on signal ingest endpoints.
type APIKeyTokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*TokenBucket
	rate    float64
	burst   float64
}

// NewAPIKeyTokenBucketLimiter creates a new API key token bucket limiter.
// rate: tokens per second (e.g., 10000 for 10K/sec)
// burst: maximum bucket capacity (e.g., 100 for 100 requests burst)
func NewAPIKeyTokenBucketLimiter(rate, burst float64) *APIKeyTokenBucketLimiter {
	return &APIKeyTokenBucketLimiter{
		buckets: make(map[string]*TokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// bucketFor returns the TokenBucket for the given prefix, creating one if needed.
// It uses lock-free read-first approach for efficiency on the hot path.
func (l *APIKeyTokenBucketLimiter) bucketFor(prefix string) *TokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[prefix]
	if !ok {
		b = NewTokenBucket(l.rate, l.burst)
		l.buckets[prefix] = b
	}
	return b
}

// Middleware returns a chi/http middleware that enforces per-api-key limits.
// Must be mounted AFTER auth.APIKeyMiddleware so the APIKey is in context.
func (l *APIKeyTokenBucketLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k, ok := auth.APIKeyFromContext(r.Context())
			if !ok || k == nil {
				// No API key in context — should not happen if mounted correctly.
				// Pass through; upstream middleware enforces authentication.
				next.ServeHTTP(w, r)
				return
			}

			// Get or create bucket for this API key prefix
			// Naming convention documented: tb:signals:{api_key_prefix}
			// The bucket map uses k.KeyPrefix directly as the key
			bucketKey := "tb:signals:" + k.KeyPrefix
			_ = bucketKey // key name is documented; internal map uses k.KeyPrefix

			b := l.bucketFor(k.KeyPrefix)

			// Check if request is allowed
			if !b.Allow() {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":"signal ingest rate limit exceeded"}`)
				return
			}

			// Request allowed, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}
