package resilience

import (
	"fmt"
	"net/http"
	"time"
)

type KeyFunc func(*http.Request) string

func Limit(rl *RedisRateLimiter, keyFn KeyFunc, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				// No key derivable (e.g., anonymous on an auth'd route) — let downstream handle
				next.ServeHTTP(w, r)
				return
			}

			allowed, retry, err := rl.Allow(r.Context(), key, limit, window)
			if err != nil {
				// Fail-open but set a header so ops can see degradation
				w.Header().Set("X-RateLimit-Degraded", "true")
			}

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retry))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"error":"rate limit exceeded","retry_after":%d}`, retry)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
