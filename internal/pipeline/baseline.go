package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/argusxdr/argus/internal/baseline"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// BaselineScorer is a Processor that reads cached baseline profiles from Redis
// and computes z-scores for incoming signals. It enriches signals with baseline
// deviation metrics (stored in enrichment.baseline_deviation).
//
// BaselineScorer is non-fatal: if Redis is unavailable or a profile is missing,
// it silently returns the signal unchanged. This allows the pipeline to continue
// even if baseline profiling is not yet available.
type BaselineScorer struct {
	redis   *redis.Client
	logger  *zap.Logger
	calc    *baseline.ProfileCalculator
	metrics *BaselineScorerMetrics
}

// BaselineScorerMetrics tracks baseline scoring statistics
type BaselineScorerMetrics struct {
	scored *prometheus.CounterVec  // argus_pipeline_baseline_scored_total{app_id, layer}
	errors prometheus.Counter      // argus_pipeline_baseline_errors_total
}

// NewBaselineScorer creates a new BaselineScorer processor.
// redis: Redis client for looking up baseline profiles.
// logger: Logger for errors and debugging.
func NewBaselineScorer(redis *redis.Client, logger *zap.Logger) *BaselineScorer {
	if logger == nil {
		logger = zap.NewNop()
	}

	metrics := &BaselineScorerMetrics{
		scored: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "argus_pipeline_baseline_scored_total",
				Help: "Total number of signals scored with baseline z-scores",
			},
			[]string{"app_id", "layer"},
		),
		errors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "argus_pipeline_baseline_errors_total",
				Help: "Total number of baseline scoring errors (non-fatal)",
			},
		),
	}

	return &BaselineScorer{
		redis:   redis,
		logger:  logger,
		calc:    baseline.NewProfileCalculator(),
		metrics: metrics,
	}
}

// Process applies baseline scoring to a signal.
// If the signal already has enrichment.baseline_deviation set, returns unchanged.
// Otherwise, extracts a metric (e.g., duration_ms), looks up the profile in Redis,
// and computes a z-score.
// Returns (sig, nil) if the signal is processed (changed or unchanged).
// Returns (sig, nil) non-fatally if profile is missing or Redis error occurs.
// Returns (nil, error) only for critical errors (e.g., signal is nil).
func (bs *BaselineScorer) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// If baseline_deviation is already set, return unchanged
	if sig.Enrichment != nil && sig.Enrichment.BaselineDeviation != nil && *sig.Enrichment.BaselineDeviation != 0 {
		return sig, nil
	}

	// Extract a metric from the signal
	metric := extractMetric(sig)
	if metric == nil {
		// No metric to score, return unchanged
		return sig, nil
	}

	// If Redis is not available, return signal unchanged (non-fatal)
	if bs.redis == nil {
		return sig, nil
	}

	// Build the Redis key for this signal's profile
	// Key format: baseline:{app_id}:{layer}:{category}
	key := fmt.Sprintf("baseline:%s:%s:%s",
		sig.Source.AppId,
		sig.Layer.String(),
		sig.Category,
	)

	// Lookup profile in Redis
	profileJSON, err := bs.redis.Get(ctx, key).Result()
	if err != nil {
		// Non-fatal: profile not found or Redis error
		if err != redis.Nil {
			bs.logger.Debug("redis lookup failed",
				zap.String("signal_id", sig.SignalId),
				zap.String("key", key),
				zap.Error(err),
			)
		}
		// Return signal unchanged
		return sig, nil
	}

	// Parse the profile from JSON
	profile := &baseline.BaselineProfile{}
	if err := json.Unmarshal([]byte(profileJSON), profile); err != nil {
		bs.logger.Debug("failed to unmarshal profile",
			zap.String("signal_id", sig.SignalId),
			zap.String("key", key),
			zap.Error(err),
		)
		// Return signal unchanged
		return sig, nil
	}

	// Compute z-score using the calculator (handles stddev=0 case)
	zscore := bs.calc.ComputeZScore(*metric, profile)

	// Create a deep copy of the signal to avoid mutation
	enriched := proto.Clone(sig).(*v1.ArgusSignal)

	// Ensure enrichment is initialized
	if enriched.Enrichment == nil {
		enriched.Enrichment = &v1.Enrichment{}
	}

	// Populate the z-score (convert float64 to float32 for proto)
	zscoreFloat32 := float32(zscore)
	enriched.Enrichment.BaselineDeviation = &zscoreFloat32

	bs.logger.Debug("baseline scored",
		zap.String("signal_id", sig.SignalId),
		zap.Float64("metric", *metric),
		zap.Float64("mean", profile.Mean),
		zap.Float64("stddev", profile.StdDev),
		zap.Float32("zscore", float32(zscore)),
	)

	// Increment metric
	if bs.metrics != nil && bs.metrics.scored != nil {
		bs.metrics.scored.WithLabelValues(sig.Source.AppId, sig.Layer.String()).Inc()
	}

	return enriched, nil
}

// Name returns the processor's name for logging and metrics.
func (bs *BaselineScorer) Name() string {
	return "BaselineScorer"
}

// RegisterMetrics registers baseline scorer metrics with Prometheus.
func (bs *BaselineScorer) RegisterMetrics(reg prometheus.Registerer) error {
	if bs.metrics == nil {
		return nil
	}
	if bs.metrics.scored != nil {
		if err := reg.Register(bs.metrics.scored); err != nil {
			return fmt.Errorf("failed to register baseline scored metric: %w", err)
		}
	}
	if err := reg.Register(bs.metrics.errors); err != nil {
		return fmt.Errorf("failed to register baseline errors metric: %w", err)
	}
	return nil
}

// extractMetric extracts a numeric metric from a signal for z-score computation.
// Tries to extract duration_ms first (most common), then layer-specific metrics.
// Returns nil if no metric is available.
func extractMetric(sig *v1.ArgusSignal) *float64 {
	// duration_ms is the primary metric
	if sig.DurationMs != nil && *sig.DurationMs > 0 {
		val := float64(*sig.DurationMs)
		return &val
	}

	// Layer-specific metrics (stub for now - proto fields not yet finalized)
	switch sig.Layer {
	case v1.Layer_L5_OUTPUT_DECODING:
		if ctx := sig.GetContextL5(); ctx != nil {
			// TODO: Extract TTFT or other L5 metrics once proto fields are finalized
		}
	case v1.Layer_L7_RAG_RETRIEVAL:
		if ctx := sig.GetContextL7(); ctx != nil {
			// TODO: Extract latency breakdown metrics once proto fields are finalized
		}
	case v1.Layer_L8_AGENTS:
		if ctx := sig.GetContextL8(); ctx != nil {
			// TODO: Extract tool latency metrics once proto fields are finalized
		}
	}

	return nil
}
