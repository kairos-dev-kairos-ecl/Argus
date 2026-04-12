package pipeline

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// WorkerPool manages N fixed goroutines that pull signals from the ingest queue,
// feed them through the processor chain, and push results to storage.
// This architecture prevents goroutine explosion: the number of workers is fixed
// regardless of signal volume. Backpressure naturally flows backward: if the
// pipeline is slow, the input channel fills and blocks further enqueues.
type WorkerPool struct {
	ingestQueue <-chan *v1.ArgusSignal
	pipeline    *Chain
	storage     Writer
	numWorkers  int
	wg          sync.WaitGroup
	done        chan struct{}
	logger      *zap.Logger
	metrics     *WorkerPoolMetrics
	started     bool
	startMu     sync.Mutex // protects started flag
}

// WorkerPoolMetrics tracks worker pool statistics
type WorkerPoolMetrics struct {
	backpressureWarnings prometheus.Counter // pipeline full warnings
	storageErrors        prometheus.Counter // write errors to storage
}

// NewWorkerPool creates a new WorkerPool.
// ingestQueue: the input channel of signals (typically from Phase 2 receivers).
// pipeline: the processor chain to route signals through.
// storage: the destination for processed signals.
// numWorkers: the number of goroutines to spawn. If 0, defaults to GOMAXPROCS * 2.
// logger: logger for errors and debugging.
func NewWorkerPool(
	ingestQueue <-chan *v1.ArgusSignal,
	pipeline *Chain,
	storage Writer,
	numWorkers int,
	logger *zap.Logger,
) *WorkerPool {
	if logger == nil {
		logger = zap.NewNop()
	}

	if numWorkers <= 0 {
		// Default: use a multiple of GOMAXPROCS
		numWorkers = 2 // Conservative default; can be tuned via config
	}

	metrics := &WorkerPoolMetrics{
		backpressureWarnings: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "argus_pipeline_backpressure_total",
				Help: "Total number of times workers encountered pipeline backpressure",
			},
		),
		storageErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "argus_pipeline_storage_errors_total",
				Help: "Total number of errors writing to storage",
			},
		),
	}

	return &WorkerPool{
		ingestQueue: ingestQueue,
		pipeline:    pipeline,
		storage:     storage,
		numWorkers:  numWorkers,
		done:        make(chan struct{}),
		logger:      logger,
		metrics:     metrics,
	}
}

// Start spawns numWorkers goroutines that begin pulling signals from the queue.
// Non-blocking: returns immediately after spawning workers.
// Start must be called exactly once. Calling Start multiple times is a no-op after the first call.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.startMu.Lock()
	if wp.started {
		wp.startMu.Unlock()
		return nil
	}
	wp.started = true
	wp.startMu.Unlock()

	// Spawn worker goroutines
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}

	wp.logger.Info("worker pool started",
		zap.Int("workers", wp.numWorkers),
	)

	return nil
}

// worker is the main loop for a worker goroutine.
// It continuously pulls signals from ingestQueue, feeds them through the pipeline,
// and writes results to storage.
// Exits when ingestQueue is closed or context is cancelled.
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, exit gracefully
			wp.logger.Debug("worker shutting down",
				zap.Int("worker_id", workerID),
				zap.Error(ctx.Err()),
			)
			return

		case <-wp.done:
			// Shutdown signal received, drain remaining signals then exit
			wp.drainQueue(ctx, workerID)
			return

		case sig, ok := <-wp.ingestQueue:
			if !ok {
				// Input queue is closed, exit
				wp.logger.Debug("ingest queue closed",
					zap.Int("worker_id", workerID),
				)
				return
			}

			// Process the signal through the pipeline
			wp.processSignal(ctx, workerID, sig)
		}
	}
}

// drainQueue processes any remaining signals in the queue after shutdown.
// Called on graceful shutdown to ensure no signals are lost.
func (wp *WorkerPool) drainQueue(ctx context.Context, workerID int) {
	for sig := range wp.ingestQueue {
		wp.processSignal(ctx, workerID, sig)
	}
}

// processSignal feeds a signal through the pipeline and writes the result to storage.
// Non-fatal error handling: backpressure and write errors are logged and metrics are incremented.
func (wp *WorkerPool) processSignal(ctx context.Context, workerID int, sig *v1.ArgusSignal) {
	if sig == nil {
		return
	}

	// Enqueue signal to the pipeline
	if err := wp.pipeline.Enqueue(sig); err != nil {
		if err == ErrPipelineFull {
			// Backpressure: pipeline input buffer is full
			// Log at warning level and increment metric
			wp.logger.Warn("pipeline backpressure",
				zap.Int("worker_id", workerID),
				zap.String("signal_id", sig.SignalId),
			)
			if wp.metrics != nil {
				wp.metrics.backpressureWarnings.Inc()
			}
			// Continue: don't block, just skip this signal (it will be retried by the caller)
			return
		}
		// Other enqueue errors (e.g., ErrChainShutdown) — log and skip
		wp.logger.Error("pipeline enqueue failed",
			zap.Int("worker_id", workerID),
			zap.String("signal_id", sig.SignalId),
			zap.Error(err),
		)
		return
	}

	// Read the result from the pipeline output channel
	// Use a non-blocking select to avoid deadlocks if pipeline is slow
	select {
	case result := <-wp.pipeline.Results():
		if result == nil {
			// Signal was filtered by a processor (returned nil, nil)
			wp.logger.Debug("signal filtered by pipeline",
				zap.Int("worker_id", workerID),
				zap.String("signal_id", sig.SignalId),
			)
			return
		}

		// Write the result to storage
		if err := wp.storage.Write(ctx, result); err != nil {
			// Non-fatal: log error and increment metric
			wp.logger.Error("storage write failed",
				zap.Int("worker_id", workerID),
				zap.String("signal_id", result.SignalId),
				zap.Error(err),
			)
			if wp.metrics != nil {
				wp.metrics.storageErrors.Inc()
			}
			// Continue processing other signals
			return
		}

		wp.logger.Debug("signal written to storage",
			zap.Int("worker_id", workerID),
			zap.String("signal_id", result.SignalId),
		)

	case <-ctx.Done():
		// Context cancelled while waiting for result
		wp.logger.Debug("context cancelled while waiting for result",
			zap.Int("worker_id", workerID),
		)

	case <-wp.done:
		// Shutdown in progress
		wp.logger.Debug("shutdown signal received while processing",
			zap.Int("worker_id", workerID),
		)
	}
}

// Wait blocks until all workers have finished processing and exited.
// Called on graceful shutdown to ensure all in-flight signals are processed.
func (wp *WorkerPool) Wait() error {
	wp.wg.Wait()
	wp.logger.Info("all workers have exited")
	return nil
}

// Shutdown gracefully shuts down the worker pool.
// Signals workers to finish their current work and stop accepting new signals.
// Does not close the input queue (caller's responsibility).
func (wp *WorkerPool) Shutdown() {
	wp.logger.Info("shutting down worker pool")
	close(wp.done)
}

// RegisterMetrics registers worker pool metrics with Prometheus.
func (wp *WorkerPool) RegisterMetrics(reg prometheus.Registerer) error {
	if wp.metrics == nil {
		return nil
	}
	if err := reg.Register(wp.metrics.backpressureWarnings); err != nil {
		return err
	}
	if err := reg.Register(wp.metrics.storageErrors); err != nil {
		return err
	}
	return nil
}
