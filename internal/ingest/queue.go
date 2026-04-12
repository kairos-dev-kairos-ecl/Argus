package ingest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
)

// ErrQueueFull is returned by Enqueue when the channel is at capacity.
var ErrQueueFull = errors.New("ingest queue full")

// Dequeuer is the interface that consumers of ingest signals must implement.
// This breaks the circular dependency between ingest and storage packages.
type Dequeuer interface {
	Dequeue(ctx context.Context) (*v1.ArgusSignal, error)
}

// Queue is a bounded, thread-safe signal buffer backed by a Go buffered channel.
// It is the single point of backpressure between receivers and the storage writer.
type Queue struct {
	ch       chan *v1.ArgusSignal
	capacity int
	accepted atomic.Int64  // total signals successfully enqueued (lifetime)
	dropped  atomic.Int64  // total signals dropped due to full queue (lifetime)
	metrics  *metrics.Ingest
	mu       sync.RWMutex  // protects depth gauge updates
}

// NewQueue creates a Queue with the given capacity.
// Recommended default: 100,000 (holds ~100K in-flight signals before backpressure kicks in).
func NewQueue(capacity int, m *metrics.Ingest) *Queue {
	return &Queue{
		ch:       make(chan *v1.ArgusSignal, capacity),
		capacity: capacity,
		metrics:  m,
	}
}

// Enqueue attempts a non-blocking send to the channel.
// Returns ErrQueueFull if the channel is full.
// NEVER blocks — callers must handle ErrQueueFull themselves (return 429/RESOURCE_EXHAUSTED).
// On success: increments accepted counter and argus_ingest_queue_depth gauge.
// On full: increments dropped counter and argus_signals_dropped_total counter.
func (q *Queue) Enqueue(sig *v1.ArgusSignal) error {
	if sig == nil {
		return errors.New("signal cannot be nil")
	}

	select {
	case q.ch <- sig:
		// Successfully enqueued
		q.accepted.Add(1)
		if q.metrics != nil && q.metrics.QueueDepth != nil {
			q.mu.RLock()
			depth := len(q.ch)
			q.mu.RUnlock()
			q.metrics.QueueDepth.Set(float64(depth))
		}
		return nil
	default:
		// Queue full
		q.dropped.Add(1)
		if q.metrics != nil && q.metrics.SignalsDropped != nil {
			q.metrics.SignalsDropped.WithLabelValues("queue_full").Inc()
		}
		return ErrQueueFull
	}
}

// Dequeue blocks until a signal is available or ctx is cancelled.
// Returns (signal, nil) on success, (nil, ctx.Err()) on cancellation.
// Used by drain workers.
func (q *Queue) Dequeue(ctx context.Context) (*v1.ArgusSignal, error) {
	select {
	case sig := <-q.ch:
		if q.metrics != nil && q.metrics.QueueDepth != nil {
			q.mu.RLock()
			depth := len(q.ch)
			q.mu.RUnlock()
			q.metrics.QueueDepth.Set(float64(depth))
		}
		return sig, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Depth returns the current number of signals in the queue (non-blocking snapshot).
func (q *Queue) Depth() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.ch)
}

// Stats returns accepted and dropped counts since server start.
func (q *Queue) Stats() (accepted, dropped int64) {
	return q.accepted.Load(), q.dropped.Load()
}

// Close gracefully closes the queue channel.
// Drain workers should have already stopped before closing.
func (q *Queue) Close() error {
	close(q.ch)
	return nil
}
