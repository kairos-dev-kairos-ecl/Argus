package baseline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/argusxdr/argus/internal/metrics"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// BaselineEngine computes baseline profiles asynchronously.
// It runs a background goroutine on a fixed 10-minute interval,
// querying ClickHouse for signal statistics and caching results in Redis.
// Does NOT block the ingest hot path.
type BaselineEngine struct {
	ch         driver.Conn
	pg         *pgxpool.Pool
	redis      *redis.Client
	logger     *zap.Logger
	store      *ProfileStore
	calculator *ProfileCalculator
	config     *BaselineConfig

	// Control
	ticker *time.Ticker
	done   chan struct{}
	wg     sync.WaitGroup

	// Metrics
	metricsFactory *metrics.Storage
}

// NewBaselineEngine creates a new baseline engine with the given dependencies.
func NewBaselineEngine(
	ch driver.Conn,
	pg *pgxpool.Pool,
	redis *redis.Client,
	logger *zap.Logger,
	config *BaselineConfig,
	mf *metrics.Storage,
) *BaselineEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultBaselineConfig()
	}

	return &BaselineEngine{
		ch:             ch,
		pg:             pg,
		redis:          redis,
		logger:         logger,
		store:          NewProfileStore(redis, pg, logger),
		calculator:     NewProfileCalculator(),
		config:         config,
		done:           make(chan struct{}),
		metricsFactory: mf,
	}
}

// Start launches the background baseline computation goroutine.
// It returns immediately and runs computation on a fixed interval.
// Does not block the ingest hot path.
func (be *BaselineEngine) Start(ctx context.Context) error {
	be.ticker = time.NewTicker(be.config.ComputeInterval)
	be.wg.Add(1)

	go func() {
		defer be.wg.Done()
		be.logger.Info("baseline engine started", zap.Duration("interval", be.config.ComputeInterval))

		// Run once immediately on startup
		be.compute(ctx)

		// Then run on interval
		for {
			select {
			case <-be.done:
				be.logger.Info("baseline engine stopped")
				return
			case <-be.ticker.C:
				be.compute(ctx)
			case <-ctx.Done():
				be.logger.Info("baseline engine context cancelled")
				return
			}
		}
	}()

	return nil
}

// Stop gracefully stops the baseline engine.
// Waits for in-flight computations to complete.
func (be *BaselineEngine) Stop() error {
	if be.ticker != nil {
		be.ticker.Stop()
	}
	close(be.done)

	// Wait for goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		be.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("baseline engine stop timeout")
	}
}

// compute performs one cycle of baseline profile computation.
// Queries ClickHouse for (app_id, layer, category) combinations,
// computes profiles for those with >= MinSamples, and stores results.
// Errors are logged but non-fatal (engine continues).
func (be *BaselineEngine) compute(ctx context.Context) {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		be.logger.Debug("baseline computation cycle completed", zap.Duration("duration", duration))
		if be.metricsFactory != nil {
			// Record duration in histogram (would need metrics implementation)
			_ = duration
		}
	}()

	be.logger.Debug("starting baseline computation", zap.Duration("window", be.config.HistoryWindow))

	// Query ClickHouse for signal statistics by (app_id, layer, category)
	profiles, err := be.queryProfiles(ctx)
	if err != nil {
		be.logger.Error("failed to query ClickHouse for profiles", zap.Error(err))
		return
	}

	if len(profiles) == 0 {
		be.logger.Debug("no profiles to compute (insufficient data)")
		return
	}

	be.logger.Debug("computed profiles", zap.Int("count", len(profiles)))

	// Store each profile
	for _, prof := range profiles {
		err := be.store.StoreProfile(ctx, prof.AppID, prof.Layer, prof.Category, prof.Profile)
		if err != nil {
			be.logger.Warn("failed to store profile", zap.Error(err), zap.String("app_id", prof.AppID), zap.String("layer", prof.Layer))
			// Continue processing other profiles (non-fatal)
		}
	}
}

// profileData represents a computed baseline profile with its key.
type profileData struct {
	AppID    string
	Layer    string
	Category string
	Profile  *BaselineProfile
}

// queryProfiles queries ClickHouse for signal statistics.
// Returns a list of profiles for all (app_id, layer, category) combinations
// with sufficient historical data.
func (be *BaselineEngine) queryProfiles(ctx context.Context) ([]profileData, error) {
	// Calculate the lookback window
	windowStart := time.Now().Add(-be.config.HistoryWindow)

	// Query: Get signal statistics by (app_id, layer, category) with >= MinSamples
	// This is a simplified query; actual implementation depends on ClickHouse schema
	query := fmt.Sprintf(`
		SELECT
			app_id,
			layer,
			category,
			count() as count,
			avg(CAST(duration_ms AS Float64)) as mean,
			stddevPop(CAST(duration_ms AS Float64)) as stddev,
			min(CAST(duration_ms AS Float64)) as min,
			max(CAST(duration_ms AS Float64)) as max
		FROM signals
		WHERE timestamp >= fromUnixTimestamp(%d)
		GROUP BY app_id, layer, category
		HAVING count() >= %d
	`, windowStart.Unix(), be.config.MinSamples)

	rows, err := be.ch.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []profileData

	for rows.Next() {
		var appID, layer, category string
		var count int64
		var mean, stddev, min, max float64

		if err := rows.Scan(&appID, &layer, &category, &count, &mean, &stddev, &min, &max); err != nil {
			be.logger.Warn("failed to scan row", zap.Error(err))
			continue
		}

		profile := &BaselineProfile{
			SampleCount: int32(count),
			Mean:        mean,
			StdDev:      stddev,
			Min:         min,
			Max:         max,
		}

		results = append(results, profileData{
			AppID:    appID,
			Layer:    layer,
			Category: category,
			Profile:  profile,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return results, nil
}
