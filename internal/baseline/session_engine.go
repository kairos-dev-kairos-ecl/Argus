package baseline

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SessionBaselineConfig holds configuration for the SessionBaselineEngine.
type SessionBaselineConfig struct {
	ComputeInterval time.Duration // default 10m
	HistoryWindow   time.Duration // default 24h
	MinSessionSize  int           // default 5 signals per session
	ProfileTTL      time.Duration // default 30m (matches Redis TTL)
}

// DefaultSessionBaselineConfig returns the production-ready default configuration.
func DefaultSessionBaselineConfig() *SessionBaselineConfig {
	return &SessionBaselineConfig{
		ComputeInterval: 10 * time.Minute,
		HistoryWindow:   24 * time.Hour,
		MinSessionSize:  5,
		ProfileTTL:      30 * time.Minute,
	}
}

// SessionBaselineEngine computes session-level baseline profiles asynchronously.
// It mirrors BaselineEngine but operates at the session (conversation_id) level
// rather than the signal (app_id, layer, category) level.
//
// Compute cadence: every ComputeInterval (default 10 minutes).
// Does NOT block the ingest hot path.
type SessionBaselineEngine struct {
	ch     driver.Conn
	pg     *pgxpool.Pool
	redis  *redis.Client
	logger *zap.Logger
	store  *SessionProfileStore
	config *SessionBaselineConfig

	ticker *time.Ticker
	done   chan struct{}
	wg     sync.WaitGroup
}

// NewSessionBaselineEngine creates a new engine with all deps injected.
func NewSessionBaselineEngine(
	ch driver.Conn,
	pg *pgxpool.Pool,
	r *redis.Client,
	log *zap.Logger,
	cfg *SessionBaselineConfig,
) *SessionBaselineEngine {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg == nil {
		cfg = DefaultSessionBaselineConfig()
	}
	return &SessionBaselineEngine{
		ch:     ch,
		pg:     pg,
		redis:  r,
		logger: log,
		store:  NewSessionProfileStore(r, pg, log),
		config: cfg,
		done:   make(chan struct{}),
	}
}

// Start launches the background session baseline computation goroutine.
// Runs compute once immediately, then on ComputeInterval ticks.
// Returns immediately — non-blocking.
func (e *SessionBaselineEngine) Start(ctx context.Context) error {
	e.ticker = time.NewTicker(e.config.ComputeInterval)
	e.wg.Add(1)

	go func() {
		defer e.wg.Done()
		e.logger.Info("session baseline engine started",
			zap.Duration("interval", e.config.ComputeInterval))

		// Run once immediately on startup
		e.compute(ctx)

		for {
			select {
			case <-e.done:
				e.logger.Info("session baseline engine stopped")
				return
			case <-e.ticker.C:
				e.compute(ctx)
			case <-ctx.Done():
				e.logger.Info("session baseline engine context cancelled")
				return
			}
		}
	}()

	return nil
}

// Stop gracefully stops the engine, waiting for in-flight computations to complete.
// Times out after 30 seconds (matches BaselineEngine.Stop).
func (e *SessionBaselineEngine) Stop() error {
	if e.ticker != nil {
		e.ticker.Stop()
	}
	close(e.done)

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("session baseline engine stop timeout")
	}
}

// sessionAgg holds raw data for one session (conversation_id) from ClickHouse.
type sessionAgg struct {
	layers    []int32
	durMS     float64
	anomalies int64
	total     int64
}

// compute queries ClickHouse for session-level data in the history window,
// aggregates per-app profiles, and persists via SessionProfileStore.
// Errors are logged but non-fatal — the engine continues on next tick.
func (e *SessionBaselineEngine) compute(ctx context.Context) {
	windowStart := time.Now().Add(-e.config.HistoryWindow)

	// Query ClickHouse for per-(app_id, conversation_id) layer sequences.
	// Groups signals within each conversation, ordered by timestamp ascending,
	// and filters out conversations with fewer than MinSessionSize signals.
	const seqQuery = `
		SELECT
			app_id,
			conversation_id,
			groupArray(layer)                                                          AS layers,
			(toFloat64(max(timestamp)) - toFloat64(min(timestamp))) * 1000            AS dur_ms,
			countIf(enrich_baseline_deviation > 2.0)                                  AS anomalies,
			count()                                                                    AS total
		FROM (
			SELECT app_id, conversation_id, layer, timestamp, enrich_baseline_deviation
			FROM signals
			WHERE timestamp >= ? AND conversation_id IS NOT NULL
			ORDER BY timestamp ASC
		)
		GROUP BY app_id, conversation_id
		HAVING total >= ?`

	rows, err := e.ch.Query(ctx, seqQuery, windowStart, e.config.MinSessionSize)
	if err != nil {
		e.logger.Error("session baseline query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	// Bucket sessions by app_id for per-app profile computation.
	byApp := map[string][]sessionAgg{}

	for rows.Next() {
		var appID, convID string
		var layers []uint8 // ClickHouse layer is UInt8
		var durMS float64
		var anomalies, total int64

		if err := rows.Scan(&appID, &convID, &layers, &durMS, &anomalies, &total); err != nil {
			e.logger.Warn("session baseline scan failed", zap.Error(err))
			continue
		}

		// Deduplicate layers preserving first-seen order to get activation sequence.
		seen := map[int32]bool{}
		seq := make([]int32, 0, len(layers))
		for _, l := range layers {
			il := int32(l)
			if !seen[il] {
				seen[il] = true
				seq = append(seq, il)
			}
		}
		byApp[appID] = append(byApp[appID], sessionAgg{
			layers:    seq,
			durMS:     durMS,
			anomalies: anomalies,
			total:     total,
		})
	}

	if err := rows.Err(); err != nil {
		e.logger.Error("session baseline rows error", zap.Error(err))
		return
	}

	now := time.Now()
	for appID, sessions := range byApp {
		prof := buildSessionProfile(appID, sessions, now, e.config.ProfileTTL)
		if err := e.store.Save(ctx, prof); err != nil {
			e.logger.Warn("save session profile failed",
				zap.String("app_id", appID),
				zap.Error(err))
		}
	}
}

// buildSessionProfile aggregates per-session data into one SessionProfile for an app.
//
// LayerActivationSequence = modal sequence (most common across sessions for this app).
// SessionDurP50/P95 = percentiles of session durations.
// AnomalyRate = sum(anomalies) / sum(total signals across all sessions).
// SampleCount = number of sessions contributing to this profile.
func buildSessionProfile(appID string, sessions []sessionAgg, now time.Time, ttl time.Duration) *SessionProfile {
	if len(sessions) == 0 {
		return &SessionProfile{
			AppID:       appID,
			ComputedAt:  now,
			ExpiresAt:   now.Add(ttl),
			LayerDwellMS: map[string]float64{},
		}
	}

	// Determine modal layer sequence (most common sequence string representation).
	counts := map[string]int{}
	decode := map[string][]int32{}
	var totalAnom, totalCount int64
	durs := make([]float64, 0, len(sessions))

	for _, s := range sessions {
		key := fmt.Sprintf("%v", s.layers)
		counts[key]++
		decode[key] = s.layers
		totalAnom += s.anomalies
		totalCount += s.total
		durs = append(durs, s.durMS)
	}

	var modalKey string
	var modalCount int
	for k, c := range counts {
		if c > modalCount {
			modalCount = c
			modalKey = k
		}
	}

	// Sort durations for percentile computation.
	sort.Float64s(durs)
	p50 := sessionPercentile(durs, 0.50)
	p95 := sessionPercentile(durs, 0.95)

	rate := 0.0
	if totalCount > 0 {
		rate = float64(totalAnom) / float64(totalCount)
	}

	return &SessionProfile{
		AppID:                   appID,
		LayerActivationSequence: decode[modalKey],
		SessionDurP50MS:         p50,
		SessionDurP95MS:         p95,
		LayerDwellMS:            map[string]float64{},
		AnomalyRate:             rate,
		SampleCount:             int32(len(sessions)),
		ComputedAt:              now,
		ExpiresAt:               now.Add(ttl),
	}
}

// sessionPercentile returns the p-th percentile value from a pre-sorted float64 slice.
// Returns 0 for empty input.
func sessionPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
