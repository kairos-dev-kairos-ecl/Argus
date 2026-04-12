package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis wraps go-redis/v9 and provides typed helpers for Argus ephemeral state.
// It manages trace correlation, rate limiting, and deduplication caches.
type Redis struct {
	client *redis.Client
}

// NewRedis creates a Redis client and pings the server to verify connectivity.
// Fail-fast: returns error immediately if Redis is unreachable.
//
// addr format: "host:port"
// Example: "localhost:6379"
func NewRedis(ctx context.Context, addr string) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Verify connectivity with a ping
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis at %s: %w", addr, err)
	}

	return &Redis{
		client: client,
	}, nil
}

// SetTraceSignal records a signal_id within a trace for correlation purposes.
// Uses Redis HSET with a trace:{trace_id} key and a 15-minute TTL.
// Field names are signal_id values, values are the timestamp.
func (r *Redis) SetTraceSignal(ctx context.Context, traceID, signalID string, ts time.Time) error {
	key := fmt.Sprintf("trace:%s", traceID)

	// HSET trace:{trace_id} {signal_id} {timestamp}
	if err := r.client.HSet(ctx, key, signalID, ts.Unix()).Err(); err != nil {
		return fmt.Errorf("failed to set trace signal: %w", err)
	}

	// Set TTL to 15 minutes
	if err := r.client.Expire(ctx, key, 15*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to set trace TTL: %w", err)
	}

	return nil
}

// GetTraceSignals returns all signal_id values recorded for a trace.
// Returns an empty slice if the trace key does not exist.
func (r *Redis) GetTraceSignals(ctx context.Context, traceID string) ([]string, error) {
	key := fmt.Sprintf("trace:%s", traceID)

	// HGETALL trace:{trace_id} → map[string]string
	result, err := r.client.HKeys(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get trace signals: %w", err)
	}

	return result, nil
}

// IncrRateLimit increments a sliding window counter for rate limiting.
// Uses a key format: ratelimit:{key}, increments, and sets a TTL equal to the window duration.
// Returns the current count after incrementing.
func (r *Redis) IncrRateLimit(ctx context.Context, key string, window time.Duration) (int64, error) {
	rateKey := fmt.Sprintf("ratelimit:%s", key)

	// INCR ratelimit:{key}
	count, err := r.client.Incr(ctx, rateKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	// Set TTL to the window duration (only on first increment)
	if count == 1 {
		if err := r.client.Expire(ctx, rateKey, window).Err(); err != nil {
			return 0, fmt.Errorf("failed to set rate limit TTL: %w", err)
		}
	}

	return count, nil
}

// CheckDedup checks if a signal_id has been seen within a deduplication window.
// Uses SET NX EX pattern for atomic operation: returns true if already seen, false if first occurrence.
// Also sets the key in Redis with the provided TTL for future checks.
func (r *Redis) CheckDedup(ctx context.Context, signalID string, ttl time.Duration) (bool, error) {
	dedupKey := fmt.Sprintf("dedup:%s", signalID)

	// SET NX EX pattern: SET key 1 NX EX {ttl}
	// Returns OK if set (first occurrence), nil if key already exists (duplicate)
	result, err := r.client.SetNX(ctx, dedupKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check dedup: %w", err)
	}

	// result is true if the key was set (first occurrence), false if already existed (duplicate)
	// We return the inverse: already seen = !result
	return !result, nil
}

// Close closes the Redis connection.
func (r *Redis) Close() error {
	return r.client.Close()
}

// Client returns the underlying redis.Client for advanced operations.
func (r *Redis) Client() *redis.Client {
	return r.client
}

// DeleteKey removes a key from Redis (used for testing or manual cleanup).
func (r *Redis) DeleteKey(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// FlushAll removes all keys from Redis (use with caution, typically only in tests).
func (r *Redis) FlushAll(ctx context.Context) error {
	return r.client.FlushAll(ctx).Err()
}
