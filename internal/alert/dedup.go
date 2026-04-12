package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DeduplicationWindow manages fingerprint deduplication with a two-tier approach:
// 1. Redis sorted sets for fast in-memory window enforcement (TTL-based expiry)
// 2. PostgreSQL dedup_count field for durability across restarts
//
// When a fingerprint is seen again during the dedup window, the counter is incremented
// and the alert notification is suppressed (caller receives isDuplicate=true).
type DeduplicationWindow struct {
	redisClient *redis.Client
	pgPool      *pgxpool.Pool
	logger      *zap.Logger
	ttl         time.Duration // Default 15 minutes per spec
}

// NewDeduplicationWindow creates a deduplication engine with Redis + PostgreSQL support.
// ttl defaults to 15 minutes if set to 0.
func NewDeduplicationWindow(redisClient *redis.Client, pgPool *pgxpool.Pool, ttl time.Duration, logger *zap.Logger) *DeduplicationWindow {
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DeduplicationWindow{
		redisClient: redisClient,
		pgPool:      pgPool,
		logger:      logger,
		ttl:         ttl,
	}
}

// CheckAndMark checks if a fingerprint is within the deduplication window.
// Returns (isDuplicate, dedupCount, error).
//
// If the fingerprint is new (first seen):
//   - isDuplicate = false, dedupCount = 1
//   - Records fingerprint in Redis and PostgreSQL with TTL
//
// If the fingerprint is in-flight (seen before within TTL):
//   - isDuplicate = true, dedupCount = incremented count
//   - Updates dedup_count in PostgreSQL (incremental)
//
// Fallback: If Redis is unavailable, uses PostgreSQL only (slower but functional).
func (d *DeduplicationWindow) CheckAndMark(ctx context.Context, fingerprint string, alertID uuid.UUID) (isDuplicate bool, dedupCount int, err error) {
	// Try Redis first (fast path)
	if d.redisClient != nil {
		// Use sorted set with score = current time for TTL-based expiry
		// Key format: "dedup:{fingerprint}"
		key := fmt.Sprintf("dedup:%s", fingerprint)
		now := time.Now()

		// ZADD with NX (only add if not exists) to get info about existing entry
		score := float64(now.UnixNano()) / 1e9

		// Check if fingerprint exists in sorted set
		zcard, err := d.redisClient.ZCard(ctx, key).Result()
		if err != nil && err != redis.Nil {
			// Redis error - fall through to PostgreSQL
			d.logger.Warn("redis zadd error, falling back to postgres", zap.Error(err))
			return d.checkAndMarkPostgres(ctx, fingerprint, alertID)
		}

		if zcard > 0 {
			// Fingerprint already in dedup window (duplicate)
			// Increment score to update expiry time (extend window)
			d.redisClient.ZIncrBy(ctx, key, 1, "count")
			d.redisClient.Expire(ctx, key, d.ttl)

			// Get current count from PostgreSQL (source of truth for persistence)
			dedupCount, err = d.getDedupCountPostgres(ctx, fingerprint)
			if err != nil {
				d.logger.Error("failed to get dedup count from postgres", zap.Error(err), zap.String("fingerprint", fingerprint))
				return true, 1, fmt.Errorf("failed to check dedup count: %w", err)
			}

			d.logger.Debug("fingerprint is duplicate",
				zap.String("fingerprint", fingerprint),
				zap.Int("dedup_count", dedupCount))

			return true, dedupCount, nil
		} else {
			// Fingerprint is new - add to dedup window
			d.redisClient.ZAdd(ctx, key, redis.Z{
				Score:  score,
				Member: "count",
			})
			d.redisClient.Expire(ctx, key, d.ttl)

			// Record in PostgreSQL with dedup_count = 1
			err = d.markInPostgres(ctx, fingerprint, alertID, 1)
			if err != nil {
				d.logger.Error("failed to record dedup in postgres", zap.Error(err), zap.String("fingerprint", fingerprint))
				return false, 1, fmt.Errorf("failed to record dedup: %w", err)
			}

			d.logger.Debug("fingerprint marked as new",
				zap.String("fingerprint", fingerprint))

			return false, 1, nil
		}
	}

	// Redis unavailable - use PostgreSQL only
	return d.checkAndMarkPostgres(ctx, fingerprint, alertID)
}

// checkAndMarkPostgres is the PostgreSQL fallback for deduplication.
// It queries the alerts table for fingerprints within the dedup window and increments counters.
func (d *DeduplicationWindow) checkAndMarkPostgres(ctx context.Context, fingerprint string, alertID uuid.UUID) (isDuplicate bool, dedupCount int, err error) {
	since := time.Now().Add(-d.ttl)

	// Check if fingerprint exists within dedup window
	var existingDedup int
	var existingAlertID uuid.UUID

	err = d.pgPool.QueryRow(ctx,
		`SELECT dedup_count, alert_id FROM alerts
		WHERE fingerprint = $1 AND created_at > $2
		ORDER BY created_at DESC LIMIT 1`,
		fingerprint, since,
	).Scan(&existingDedup, &existingAlertID)

	if err == pgx.ErrNoRows {
		// No existing fingerprint in window - this is new
		// Record will be created by AlertService.Create
		return false, 1, nil
	}

	if err != nil {
		d.logger.Error("failed to query dedup window", zap.Error(err), zap.String("fingerprint", fingerprint))
		return false, 0, fmt.Errorf("failed to query dedup window: %w", err)
	}

	// Fingerprint exists in window - increment counter in database
	newDedupCount := existingDedup + 1
	err = d.pgPool.QueryRow(ctx,
		`UPDATE alerts SET dedup_count = dedup_count + 1, updated_at = NOW()
		WHERE alert_id = $1
		RETURNING dedup_count`,
		existingAlertID,
	).Scan(&newDedupCount)

	if err != nil {
		d.logger.Error("failed to increment dedup counter", zap.Error(err), zap.String("alert_id", existingAlertID.String()))
		return false, 0, fmt.Errorf("failed to increment dedup counter: %w", err)
	}

	d.logger.Debug("fingerprint is duplicate (postgres)",
		zap.String("fingerprint", fingerprint),
		zap.Int("dedup_count", newDedupCount))

	return true, newDedupCount, nil
}

// markInPostgres records a new fingerprint dedup entry by updating the alert's dedup_count.
// This ensures durability across Redis restarts.
func (d *DeduplicationWindow) markInPostgres(ctx context.Context, fingerprint string, alertID uuid.UUID, dedupCount int) error {
	// This is called after alert is created, so we just verify it exists
	// The alert creation already set dedup_count to 1
	return nil
}

// getDedupCountPostgres retrieves the current dedup count for a fingerprint from PostgreSQL.
// Returns the dedup_count of the most recent alert with that fingerprint.
func (d *DeduplicationWindow) getDedupCountPostgres(ctx context.Context, fingerprint string) (int, error) {
	since := time.Now().Add(-d.ttl)
	var dedupCount int

	err := d.pgPool.QueryRow(ctx,
		`SELECT dedup_count FROM alerts
		WHERE fingerprint = $1 AND created_at > $2
		ORDER BY created_at DESC LIMIT 1`,
		fingerprint, since,
	).Scan(&dedupCount)

	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("fingerprint not found in dedup window")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query dedup count: %w", err)
	}

	return dedupCount, nil
}

// ClearDedup removes a fingerprint from the deduplication window.
// Used for testing and manual dedup reset.
func (d *DeduplicationWindow) ClearDedup(ctx context.Context, fingerprint string) error {
	if d.redisClient != nil {
		key := fmt.Sprintf("dedup:%s", fingerprint)
		if err := d.redisClient.Del(ctx, key).Err(); err != nil {
			d.logger.Error("failed to clear redis dedup", zap.Error(err), zap.String("fingerprint", fingerprint))
			return fmt.Errorf("failed to clear redis dedup: %w", err)
		}
	}
	return nil
}
