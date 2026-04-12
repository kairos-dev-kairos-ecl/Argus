package pipeline

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Writer is an interface for persisting signals to storage.
// Implemented by storage backends (ClickHouse, etc.).
// Note: This interface is also defined in internal/ingest/workers.go to avoid circular imports.
// Updates to this interface should be kept in sync.
type Writer interface {
	// Write persists a signal to storage.
	// Returns nil on success, error on failure.
	Write(ctx context.Context, sig *v1.ArgusSignal) error
}

// Router is a Processor that hands enriched signals to the storage layer for persistence.
// It is the final processor in the pipeline. Errors are returned (not swallowed),
// allowing WorkerPool to decide fatality. On storage write failure, the signal is NOT forwarded downstream.
type Router struct {
	storage Writer
	logger  *zap.Logger
	metrics *RouterMetrics
}

// RouterMetrics tracks routing statistics
type RouterMetrics struct {
	signalsWritten prometheus.Counter
	writeErrors    prometheus.Counter
}

// NewRouter creates a new Router processor.
func NewRouter(storage Writer, logger *zap.Logger) *Router {
	if logger == nil {
		logger = zap.L()
	}

	metrics := &RouterMetrics{
		signalsWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_router_signals_written_total",
			Help: "Total number of signals successfully written to storage",
		}),
		writeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_router_errors_total",
			Help: "Total number of storage write errors in router",
		}),
	}

	return &Router{
		storage: storage,
		logger:  logger,
		metrics: metrics,
	}
}

// Process writes a signal to storage.
// Returns (nil, error) if storage write fails or storage is nil.
// Returns (sig, nil) on success.
// Note: Router returns (nil, error) on failure (signal is not forwarded downstream).
func (r *Router) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// Check if storage is configured
	if r.storage == nil {
		r.logger.Error("router storage is nil, cannot persist signal",
			zap.String("signal_id", sig.SignalId),
		)
		r.metrics.writeErrors.Inc()
		return nil, fmt.Errorf("storage writer is nil")
	}

	// Attempt to write signal to storage
	if err := r.storage.Write(ctx, sig); err != nil {
		r.metrics.writeErrors.Inc()
		r.logger.Error("failed to write signal to storage",
			zap.String("signal_id", sig.SignalId),
			zap.Error(err),
		)
		// Return nil, error — signal is not forwarded downstream
		// WorkerPool will handle the error
		return nil, err
	}

	// Success
	r.metrics.signalsWritten.Inc()
	r.logger.Debug("signal written to storage",
		zap.String("signal_id", sig.SignalId),
	)
	return sig, nil
}

// Name returns the processor's name for logging and metrics.
func (r *Router) Name() string {
	return "Router"
}

// RegisterMetrics registers router metrics with Prometheus.
func (r *Router) RegisterMetrics(reg prometheus.Registerer) error {
	if err := reg.Register(r.metrics.signalsWritten); err != nil {
		return fmt.Errorf("failed to register signals written metric: %w", err)
	}
	if err := reg.Register(r.metrics.writeErrors); err != nil {
		return fmt.Errorf("failed to register write errors metric: %w", err)
	}
	return nil
}
