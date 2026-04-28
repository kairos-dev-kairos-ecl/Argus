package resilience

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRateLimiter struct {
	client  *redis.Client
	counter atomic.Int64 // For generating unique request IDs
}

func NewRedisRateLimiter(c *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: c}
}

// Allow returns (allowed, retryAfterSec, err).
// On Redis error, fails open: (true, 0, err) so caller can log and proceed.
// Uses a sliding-window counter with sorted set to track request timestamps.
func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if r == nil || r.client == nil {
		return true, 0, nil
	}

	now := time.Now()
	nowMs := now.UnixMilli()
	windowStartMs := nowMs - window.Milliseconds()

	pipe := r.client.TxPipeline()
	// Remove entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStartMs))
	// Add current request (use atomic counter for guaranteed uniqueness)
	member := fmt.Sprintf("%d-%d", nowMs, r.counter.Add(1))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(nowMs), Member: member})
	// Get cardinality
	countCmd := pipe.ZCard(ctx, key)
	// Set expiry to 2x window to ensure cleanup
	pipe.Expire(ctx, key, window*2)
	// Get oldest entry in window for retry calculation
	oldestCmd := pipe.ZRangeWithScores(ctx, key, 0, 0)

	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err // fail-open on Redis error
	}

	count := countCmd.Val()
	if count <= int64(limit) {
		return true, 0, nil
	}

	// Limit exceeded — compute retry-after from oldest entry
	// The oldest entry will leave the window at (oldestMs + window - now) ms
	retry := 1
	if oldest := oldestCmd.Val(); len(oldest) > 0 {
		oldestMs := int64(oldest[0].Score)
		retryMs := oldestMs + window.Milliseconds() - nowMs
		if retryMs < 1 {
			retryMs = 1
		}
		retry = int(math.Ceil(float64(retryMs) / 1000.0))
		if retry < 1 {
			retry = 1
		}
	}
	return false, retry, nil
}
