package notify

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DispatcherConfig holds configuration for the AlertDispatcher.
type DispatcherConfig struct {
	// WorkerCount is the number of concurrent workers processing alerts.
	// Default: 4, Recommended: 4-8 based on CPU cores and I/O latency.
	WorkerCount int

	// QueueCapacity is the buffer size for pending alerts.
	// Default: 10000. Excess alerts are rejected with an error.
	QueueCapacity int

	// SendTimeout is the context timeout for individual Send operations.
	// Default: 30 seconds.
	SendTimeout time.Duration

	// GracefulShutdownTimeout is the time to wait for workers to finish draining.
	// Default: 30 seconds.
	GracefulShutdownTimeout time.Duration
}

// DefaultDispatcherConfig returns a dispatcher config with sensible defaults.
func DefaultDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		WorkerCount:             4,
		QueueCapacity:           10000,
		SendTimeout:             30 * time.Second,
		GracefulShutdownTimeout: 30 * time.Second,
	}
}

// DispatchJob represents a single alert dispatch job.
type DispatchJob struct {
	AlertID      uuid.UUID
	Alert        interface{} // Generic alert object (could be *alert.Alert or custom type)
	Targets      []string    // Adapter names to dispatch to (e.g., ["slack", "pagerduty"])
	Notification *NotificationRequest
}

// DispatchResult holds the result of dispatching to one adapter.
type DispatchResult struct {
	AlertID     uuid.UUID
	AdapterName string
	Response    *NotificationResponse
	Error       error
	Timestamp   time.Time
}

// AlertDispatcher manages the dispatch queue and worker pool.
// It ensures at most config.WorkerCount goroutines are processing alerts.
// Backpressure flows backward when the queue is full.
type AlertDispatcher struct {
	config      *DispatcherConfig
	registry    *AdapterRegistry
	queue       chan *DispatchJob
	logger      *zap.Logger
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	shutdownOnce sync.Once

	// Metrics (lock-free atomic counters)
	accepted   atomic.Uint64 // Total jobs accepted
	failed     atomic.Uint64 // Total jobs failed (no successful sends)
	successful atomic.Uint64 // Total jobs with ≥1 successful send
	dropped    atomic.Uint64 // Total jobs rejected due to full queue

	// Result channel for test/observability purposes
	results chan *DispatchResult
}

// NewAlertDispatcher creates a new AlertDispatcher with the given config.
func NewAlertDispatcher(config *DispatcherConfig, registry *AdapterRegistry, logger *zap.Logger) (*AlertDispatcher, error) {
	if config == nil {
		config = DefaultDispatcherConfig()
	}
	if registry == nil {
		return nil, fmt.Errorf("adapter registry cannot be nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	if config.WorkerCount <= 0 {
		return nil, fmt.Errorf("worker count must be > 0")
	}
	if config.QueueCapacity <= 0 {
		return nil, fmt.Errorf("queue capacity must be > 0")
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &AlertDispatcher{
		config:   config,
		registry: registry,
		queue:    make(chan *DispatchJob, config.QueueCapacity),
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		results:  make(chan *DispatchResult, config.WorkerCount*2), // Small buffer for results
	}

	// Start fixed worker pool
	for i := 0; i < config.WorkerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}

	return d, nil
}

// Dispatch enqueues an alert for dispatch.
// Returns error if the queue is full or dispatcher is shutting down.
func (d *AlertDispatcher) Dispatch(job *DispatchJob) error {
	if job == nil {
		return fmt.Errorf("dispatch job cannot be nil")
	}

	d.accepted.Add(1)

	select {
	case d.queue <- job:
		return nil
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher is shutting down")
	default:
		d.dropped.Add(1)
		return fmt.Errorf("dispatch queue full (capacity: %d)", d.config.QueueCapacity)
	}
}

// worker is the worker goroutine that processes jobs from the queue.
func (d *AlertDispatcher) worker(id int) {
	defer d.wg.Done()

	for {
		select {
		case job := <-d.queue:
			d.processJob(job)
		case <-d.ctx.Done():
			// Drain remaining jobs on shutdown
			for {
				select {
				case job := <-d.queue:
					d.processJob(job)
				default:
					return
				}
			}
		}
	}
}

// processJob sends the notification to all target adapters.
func (d *AlertDispatcher) processJob(job *DispatchJob) {
	if job == nil || job.Notification == nil {
		return
	}

	successCount := 0
	failureCount := 0

	for _, adapterName := range job.Targets {
		notifier, exists := d.registry.Get(adapterName)
		if !exists {
			d.logger.Warn("adapter not found in registry",
				zap.String("alert_id", job.AlertID.String()),
				zap.String("adapter", adapterName))
			failureCount++
			continue
		}

		// Create a context with timeout for this send operation
		ctx, cancel := context.WithTimeout(d.ctx, d.config.SendTimeout)
		response, err := notifier.Send(ctx, job.Notification)
		cancel()

		result := &DispatchResult{
			AlertID:     job.AlertID,
			AdapterName: adapterName,
			Response:    response,
			Error:       err,
			Timestamp:   time.Now(),
		}

		if err != nil {
			d.logger.Error("failed to send notification",
				zap.String("alert_id", job.AlertID.String()),
				zap.String("adapter", adapterName),
				zap.Error(err))
			failureCount++
		} else {
			d.logger.Debug("notification sent successfully",
				zap.String("alert_id", job.AlertID.String()),
				zap.String("adapter", adapterName))
			successCount++
		}

		// Send result to results channel (non-blocking)
		select {
		case d.results <- result:
		default:
			// Results channel full, drop it (acceptable for high volume)
		}
	}

	// Update success/failure counters
	if failureCount > 0 && successCount == 0 {
		d.failed.Add(1)
	} else if successCount > 0 {
		d.successful.Add(1)
	}
}

// ResultsChan returns a channel to observe dispatch results in real-time.
// Useful for testing and observability.
func (d *AlertDispatcher) ResultsChan() <-chan *DispatchResult {
	return d.results
}

// Shutdown gracefully stops the dispatcher, waiting for in-flight jobs to complete.
func (d *AlertDispatcher) Shutdown(ctx context.Context) error {
	var shutdownErr error
	d.shutdownOnce.Do(func() {
		// Signal workers to stop accepting new jobs
		d.cancel()

		// Wait for workers to drain queue, with timeout
		done := make(chan struct{})
		go func() {
			d.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			d.logger.Info("dispatcher shut down gracefully")
		case <-ctx.Done():
			shutdownErr = fmt.Errorf("shutdown timeout exceeded")
			d.logger.Error("dispatcher shutdown timeout", zap.Error(shutdownErr))
		}

		// Close results channel
		close(d.results)
	})

	return shutdownErr
}

// Stats returns current dispatch statistics.
func (d *AlertDispatcher) Stats() map[string]uint64 {
	return map[string]uint64{
		"accepted":   d.accepted.Load(),
		"successful": d.successful.Load(),
		"failed":     d.failed.Load(),
		"dropped":    d.dropped.Load(),
		"queue_len":  uint64(len(d.queue)),
		"queue_cap":  uint64(d.config.QueueCapacity),
	}
}
