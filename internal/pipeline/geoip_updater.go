package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// GeoIPUpdater manages periodic database updates and hot-reloads for the GeoIP enricher.
// It runs a background goroutine that checks for database updates at a configurable interval
// (default: daily at 2 AM UTC) and reloads the database if updates are available.
type GeoIPUpdater struct {
	enricher      *GeoIPEnricher
	logger        *zap.Logger
	metrics       *UpdaterMetrics
	stopChan      chan struct{}
	wg            sync.WaitGroup
	checkInterval time.Duration
	checkTime     time.Time // time of day to run the check (2 AM UTC by default)
	mu            sync.RWMutex
	running       bool
}

// UpdaterMetrics tracks GeoIP update operations
type UpdaterMetrics struct {
	attemptsTotal  prometheus.Counter
	successesTotal prometheus.Counter
	failuresTotal  prometheus.Counter
}

// NewGeoIPUpdater creates a new GeoIPUpdater for the given GeoIPEnricher.
// The updater will check for database updates daily at 2 AM UTC by default.
func NewGeoIPUpdater(enricher *GeoIPEnricher, logger *zap.Logger) *GeoIPUpdater {
	if logger == nil {
		logger = zap.L()
	}

	metrics := &UpdaterMetrics{
		attemptsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_update_attempts_total",
			Help: "Total number of GeoIP update attempts",
		}),
		successesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_update_success_total",
			Help: "Total number of successful GeoIP updates",
		}),
		failuresTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_update_failures_total",
			Help: "Total number of failed GeoIP update attempts",
		}),
	}

	return &GeoIPUpdater{
		enricher:      enricher,
		logger:        logger,
		metrics:       metrics,
		stopChan:      make(chan struct{}),
		checkInterval: 24 * time.Hour,
		checkTime:     time.Date(0, 1, 1, 2, 0, 0, 0, time.UTC), // 2 AM UTC
		running:       false,
	}
}

// Start begins the background update check goroutine.
// The goroutine will run until Stop() is called.
// Calling Start() multiple times safely (only starts once).
func (gu *GeoIPUpdater) Start(ctx context.Context) error {
	gu.mu.Lock()
	if gu.running {
		gu.mu.Unlock()
		gu.logger.Debug("GeoIPUpdater already running")
		return nil
	}
	gu.running = true
	gu.mu.Unlock()

	gu.logger.Info("starting GeoIP update scheduler")

	// Start the background goroutine
	gu.wg.Add(1)
	go gu.updateLoop(ctx)

	return nil
}

// Stop gracefully stops the background update check goroutine.
// Waits for the goroutine to finish (with a reasonable timeout).
func (gu *GeoIPUpdater) Stop() error {
	gu.mu.Lock()
	if !gu.running {
		gu.mu.Unlock()
		return nil
	}
	gu.running = false
	gu.mu.Unlock()

	gu.logger.Info("stopping GeoIP update scheduler")

	// Signal the goroutine to stop
	close(gu.stopChan)

	// Wait for goroutine to finish (with timeout)
	done := make(chan struct{})
	go func() {
		gu.wg.Wait()
		close(done)
	}()

	// Use a 5-second timeout for graceful shutdown
	select {
	case <-done:
		gu.logger.Debug("GeoIP update scheduler stopped gracefully")
		return nil
	case <-time.After(5 * time.Second):
		gu.logger.Warn("GeoIP update scheduler stop timed out")
		return fmt.Errorf("GeoIP update scheduler stop timed out")
	}
}

// updateLoop runs the background update check at the configured interval.
// This function runs in its own goroutine and terminates when stopChan is closed.
func (gu *GeoIPUpdater) updateLoop(ctx context.Context) {
	defer gu.wg.Done()

	// Calculate initial sleep duration to reach the configured check time
	nextCheckTime := gu.nextCheckTime()
	gu.logger.Info("scheduling next GeoIP update check",
		zap.Time("next_check_time", nextCheckTime),
	)

	for {
		select {
		case <-gu.stopChan:
			gu.logger.Debug("GeoIP update loop stopped")
			return

		case <-ctx.Done():
			gu.logger.Debug("GeoIP update loop context cancelled")
			return

		case <-time.After(time.Until(nextCheckTime)):
			// Time to check for updates
			gu.checkAndUpdate(ctx)

			// Calculate next check time
			nextCheckTime = gu.nextCheckTime()
			gu.logger.Info("scheduling next GeoIP update check",
				zap.Time("next_check_time", nextCheckTime),
			)
		}
	}
}

// nextCheckTime calculates when the next update check should occur.
// By default, this is the next occurrence of 2 AM UTC.
func (gu *GeoIPUpdater) nextCheckTime() time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)

	// If we've already passed 2 AM today, schedule for tomorrow
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}

	return next
}

// checkAndUpdate performs the actual update check and triggers a hot-reload if needed.
// This function is non-fatal: errors are logged but don't stop the scheduler.
// In a production implementation, this would contact MaxMind API or check for newer files.
func (gu *GeoIPUpdater) checkAndUpdate(ctx context.Context) {
	gu.metrics.attemptsTotal.Inc()

	gu.logger.Debug("starting GeoIP update check")

	// In a production implementation, this would:
	// 1. Contact MaxMind API or check local file timestamp
	// 2. Download the new database if available
	// 3. Verify the downloaded file integrity
	// 4. Call gu.enricher.UpdateDatabase(ctx, newPath)

	// For now, we implement a placeholder that demonstrates the pattern:
	// - Check if the current database is stale (>25 days)
	// - Log appropriate messages
	// - Record metrics

	// Placeholder: In real implementation, fetch latest database info from MaxMind
	// and compare with current version. For now, just perform a dry-run check.
	err := gu.performDummyUpdate(ctx)
	if err != nil {
		gu.metrics.failuresTotal.Inc()
		gu.logger.Warn("GeoIP update check failed",
			zap.Error(err),
		)
		return
	}

	gu.metrics.successesTotal.Inc()
	gu.logger.Debug("GeoIP update check completed successfully")
}

// performDummyUpdate is a placeholder implementation for the actual MaxMind update logic.
// In production, this would:
// - Download the latest GeoLite2 database from MaxMind
// - Verify file integrity (checksums)
// - Extract the .mmdb file if necessary
// - Call UpdateDatabase() for hot-reload
// For now, it simulates the pattern and returns nil (no update needed).
func (gu *GeoIPUpdater) performDummyUpdate(ctx context.Context) error {
	// Placeholder logic:
	// Real implementation would contact MaxMind or check for newer local files.
	// For testing, return nil (no update needed).
	// In production, this would:
	//   1. Fetch metadata about latest database from MaxMind
	//   2. Compare with local database version/timestamp
	//   3. If newer available:
	//      a. Download to temporary location
	//      b. Verify integrity (MD5/SHA256)
	//      c. Extract if .tar.gz
	//      d. Call ge.UpdateDatabase(ctx, tempPath)
	//   4. Clean up temporary files

	// For now, just return nil indicating no update was needed
	gu.logger.Debug("GeoIP update check: no update available (placeholder implementation)")
	return nil
}

// RegisterMetrics registers the updater's Prometheus metrics.
func (gu *GeoIPUpdater) RegisterMetrics(reg prometheus.Registerer) error {
	if err := reg.Register(gu.metrics.attemptsTotal); err != nil {
		return fmt.Errorf("failed to register geoip update attempts metric: %w", err)
	}
	if err := reg.Register(gu.metrics.successesTotal); err != nil {
		return fmt.Errorf("failed to register geoip update successes metric: %w", err)
	}
	if err := reg.Register(gu.metrics.failuresTotal); err != nil {
		return fmt.Errorf("failed to register geoip update failures metric: %w", err)
	}
	return nil
}
