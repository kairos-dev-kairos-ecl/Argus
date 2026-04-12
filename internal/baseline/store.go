package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ProfileStore handles persistence and retrieval of baseline profiles.
// It uses Redis as a fast cache (5-minute TTL) and PostgreSQL as permanent storage.
type ProfileStore struct {
	redis  *redis.Client
	pg     *pgxpool.Pool
	logger *zap.Logger
}

// NewProfileStore creates a new profile store with Redis and PostgreSQL backends.
func NewProfileStore(redis *redis.Client, pg *pgxpool.Pool, logger *zap.Logger) *ProfileStore {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ProfileStore{
		redis:  redis,
		pg:     pg,
		logger: logger,
	}
}

// StoreProfile persists a baseline profile to both PostgreSQL and Redis.
// Stores in Redis with 5-minute TTL for fast cache hits.
// Stores in PostgreSQL for permanent record and durability.
// Returns error if both backends fail; logs individual failures.
func (ps *ProfileStore) StoreProfile(ctx context.Context, appID string, layer string, category string, profile *BaselineProfile) error {
	if profile == nil {
		return fmt.Errorf("cannot store nil profile")
	}

	key := makeProfileKey(appID, layer, category)
	now := time.Now()

	// Marshal profile to JSON for storage
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		ps.logger.Error("failed to marshal profile", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	// Try Redis first (non-fatal if it fails)
	redisErr := ps.storeInRedis(ctx, key, profileJSON)
	if redisErr != nil {
		ps.logger.Warn("failed to store profile in Redis", zap.Error(redisErr), zap.String("key", key))
		// Don't return; try PostgreSQL
	}

	// Try PostgreSQL (required for durability)
	pgErr := ps.storeInPostgres(ctx, appID, layer, category, profile, now)
	if pgErr != nil {
		ps.logger.Error("failed to store profile in PostgreSQL", zap.Error(pgErr), zap.String("key", key))
		return fmt.Errorf("failed to store profile in postgres: %w", pgErr)
	}

	return nil
}

// GetProfile retrieves a baseline profile from cache or permanent storage.
// Tries Redis first (fast path); falls back to PostgreSQL if not cached.
// Returns nil if profile not found in either backend (non-fatal).
func (ps *ProfileStore) GetProfile(ctx context.Context, appID string, layer string, category string) (*BaselineProfile, error) {
	key := makeProfileKey(appID, layer, category)

	// Try Redis first
	profile, redisErr := ps.getFromRedis(ctx, key)
	if redisErr == nil && profile != nil {
		return profile, nil
	}

	if redisErr != nil && redisErr != redis.Nil {
		ps.logger.Debug("redis get error (falling back to postgres)", zap.Error(redisErr), zap.String("key", key))
	}

	// Fall back to PostgreSQL
	profile, pgErr := ps.getFromPostgres(ctx, appID, layer, category)
	if pgErr != nil {
		ps.logger.Debug("failed to retrieve profile from postgres", zap.Error(pgErr), zap.String("key", key))
		return nil, nil // Non-fatal: profile not found yet
	}

	// Cache the result in Redis for next time
	if profile != nil {
		profileJSON, _ := json.Marshal(profile)
		_ = ps.storeInRedis(ctx, key, profileJSON) // Best effort
	}

	return profile, nil
}

// storeInRedis caches the profile in Redis with 5-minute TTL.
func (ps *ProfileStore) storeInRedis(ctx context.Context, key string, profileJSON []byte) error {
	return ps.redis.Set(ctx, key, profileJSON, 5*time.Minute).Err()
}

// getFromRedis retrieves a profile from Redis cache.
func (ps *ProfileStore) getFromRedis(ctx context.Context, key string) (*BaselineProfile, error) {
	val, err := ps.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, redis.Nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var profile BaselineProfile
	if err := json.Unmarshal([]byte(val), &profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile from redis: %w", err)
	}

	return &profile, nil
}

// storeInPostgres persists the profile to the baseline_profiles table.
// Uses UPSERT (ON CONFLICT) to update existing profiles.
func (ps *ProfileStore) storeInPostgres(ctx context.Context, appID string, layer string, category string, profile *BaselineProfile, now time.Time) error {
	const query = `
		INSERT INTO baseline_profiles (app_id, layer, category, count, mean, stddev, min, max, computed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (app_id, layer, category) DO UPDATE SET
			count = EXCLUDED.count,
			mean = EXCLUDED.mean,
			stddev = EXCLUDED.stddev,
			min = EXCLUDED.min,
			max = EXCLUDED.max,
			computed_at = EXCLUDED.computed_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := ps.pg.Exec(ctx, query,
		appID, layer, category,
		profile.SampleCount, profile.Mean, profile.StdDev, profile.Min, profile.Max,
		now, now,
	)

	return err
}

// getFromPostgres retrieves a profile from the baseline_profiles table.
func (ps *ProfileStore) getFromPostgres(ctx context.Context, appID string, layer string, category string) (*BaselineProfile, error) {
	const query = `
		SELECT count, mean, stddev, min, max
		FROM baseline_profiles
		WHERE app_id = $1 AND layer = $2 AND category = $3
		LIMIT 1
	`

	row := ps.pg.QueryRow(ctx, query, appID, layer, category)

	var profile BaselineProfile
	err := row.Scan(&profile.SampleCount, &profile.Mean, &profile.StdDev, &profile.Min, &profile.Max)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

// makeProfileKey creates a Redis key for a baseline profile.
func makeProfileKey(appID string, layer string, category string) string {
	return fmt.Sprintf("baseline:%s:%s:%s", appID, layer, category)
}
