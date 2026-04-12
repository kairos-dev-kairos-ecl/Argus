package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// CorrelationTagger is a Processor that groups signals by trace_id using Redis sorted sets.
// Signals arriving within a configurable window (default 5 seconds) are tagged with each
// other's IDs in the related_signals field. This enables correlation across a distributed trace.
//
// Architecture notes:
// - Redis sorted sets store signals keyed by trace:{trace_id}
// - Members are formatted as: {signal_id}:{app_id}:{layer}
// - Scores are millisecond timestamps (UnixMilli) for temporal ordering with sufficient precision
// - TTL is set to 30 seconds (5s window + 25s buffer) to prevent unbounded key growth
// - Late-arriving signals (beyond the window) will not be correlated with earlier signals
//   but will be queryable by later signals within their window. This is acceptable because:
//   1. Most signals arrive in-order due to distributed tracing SDKs
//   2. A 5-second window is conservative for modern infrastructure
//   3. The 30-second TTL ensures eventual cleanup
// - Redis errors are non-fatal: if Redis is unavailable, signals pass through unchanged
//   without blocking the ingestion pipeline (graceful degradation)
type CorrelationTagger struct {
	redis   *redis.Client
	window  time.Duration
	logger  *zap.Logger
	metrics CorrelationMetrics
}

// CorrelationMetrics tracks correlation statistics
type CorrelationMetrics struct {
	// argus_pipeline_correlation_tagged_total Counter
	// Total number of signals processed by CorrelationTagger
	signalsTagged prometheus.Counter

	// argus_pipeline_correlation_errors_total Counter
	// Total Redis errors encountered (logged but non-fatal)
	redisErrors prometheus.Counter
}

// NewCorrelationTagger creates a new CorrelationTagger processor.
// Parameters:
//   - redis: an initialized redis.Client for sorted set operations
//   - window: the correlation window duration (typically 5 seconds)
//
// Returns a ready-to-use CorrelationTagger or an error if logger setup fails.
func NewCorrelationTagger(client *redis.Client, window time.Duration) *CorrelationTagger {
	// Create metrics without registration error handling
	// (assume metrics are registered globally elsewhere or allow duplicate)
	metrics := CorrelationMetrics{
		signalsTagged: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_correlation_tagged_total",
			Help: "Total number of signals processed by CorrelationTagger",
		}),
		redisErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_correlation_errors_total",
			Help: "Total Redis errors encountered during correlation",
		}),
	}

	return &CorrelationTagger{
		redis:   client,
		window:  window,
		logger:  zap.L(),
		metrics: metrics,
	}
}

// Process applies correlation tagging to a signal.
// If the signal has no trace_id, it is returned unchanged.
// Otherwise, the signal is added to a Redis sorted set keyed by trace_id,
// and all other signals within the correlation window are fetched and added
// to the signal's related_signals field.
func (ct *CorrelationTagger) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// If no trace_id, no correlation is possible
	if sig.TraceId == "" {
		return sig, nil
	}

	// Mark that we processed this signal (regardless of Redis outcome)
	ct.metrics.signalsTagged.Inc()

	// Add this signal to the sorted set for this trace
	// Key format: trace:{trace_id}
	// Member format: {signal_id}:{app_id}:{layer}
	// Score: millisecond timestamp (UnixMilli)
	traceKey := fmt.Sprintf("trace:%s", sig.TraceId)

	// Extract app_id and layer for member format
	appID := "unknown"
	if sig.Source != nil {
		appID = sig.Source.AppId
	}
	layer := sig.Layer.String()

	member := fmt.Sprintf("%s:%s:%s", sig.SignalId, appID, layer)

	// Convert timestamp to milliseconds for score
	// sig.Timestamp is a *timestamppb.Timestamp, which can be nil if not set
	// (though SchemaValidator should catch this)
	var scoreMs float64
	if sig.Timestamp != nil {
		scoreMs = float64(sig.Timestamp.AsTime().UnixMilli())
	} else {
		scoreMs = float64(time.Now().UnixMilli())
	}

	// Add signal to sorted set with millisecond score
	err := ct.redis.ZAdd(ctx, traceKey, redis.Z{
		Score:  scoreMs,
		Member: member,
	}).Err()

	if err != nil {
		ct.metrics.redisErrors.Inc()
		ct.logger.Warn("failed to add signal to correlation set",
			zap.String("signal_id", sig.SignalId),
			zap.String("trace_id", sig.TraceId),
			zap.Error(err),
		)
		// Non-fatal: return signal unchanged
		return sig, nil
	}

	// Set TTL on the key (30 seconds = 5s window + 25s buffer)
	ttl := 30 * time.Second
	err = ct.redis.Expire(ctx, traceKey, ttl).Err()
	if err != nil {
		ct.metrics.redisErrors.Inc()
		ct.logger.Warn("failed to set TTL on correlation key",
			zap.String("trace_id", sig.TraceId),
			zap.Error(err),
		)
		// Non-fatal: continue without TTL (key will eventually expire via Redis eviction)
	}

	// Query signals within the correlation window
	// Window is: (now - ct.window) to now (in milliseconds)
	windowMs := float64(ct.window.Milliseconds())
	minScore := scoreMs - windowMs
	maxScore := scoreMs

	results, err := ct.redis.ZRangeByScore(ctx, traceKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%g", minScore),
		Max: fmt.Sprintf("%g", maxScore),
	}).Result()

	if err != nil {
		ct.metrics.redisErrors.Inc()
		ct.logger.Warn("failed to query correlated signals",
			zap.String("signal_id", sig.SignalId),
			zap.String("trace_id", sig.TraceId),
			zap.Error(err),
		)
		// Non-fatal: continue without adding related_signals
		return sig, nil
	}

	// Extract signal IDs from results, excluding self
	var relatedSignals []string
	for _, result := range results {
		// result is in format: {signal_id}:{app_id}:{layer}
		parts := strings.Split(result, ":")
		if len(parts) > 0 {
			resultSignalID := parts[0]
			if resultSignalID != sig.SignalId {
				relatedSignals = append(relatedSignals, resultSignalID)
			}
		}
	}

	// Set related_signals on the signal
	sig.RelatedSignals = relatedSignals

	return sig, nil
}

// Name returns the processor name for logging and metrics
func (ct *CorrelationTagger) Name() string {
	return "CorrelationTagger"
}

// SetLogger sets the logger for the CorrelationTagger (optional; defaults to zap.L())
func (ct *CorrelationTagger) SetLogger(logger *zap.Logger) {
	if logger != nil {
		ct.logger = logger
	}
}
