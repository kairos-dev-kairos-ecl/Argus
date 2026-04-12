package ingest

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
)

func TestQueueEnqueue(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(100, ingestMetrics)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-1",
		TraceId:  "test-trace-1",
	}

	err := q.Enqueue(sig)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	accepted, dropped := q.Stats()
	if accepted != 1 || dropped != 0 {
		t.Errorf("Stats mismatch: expected (1, 0), got (%d, %d)", accepted, dropped)
	}

	if q.Depth() != 1 {
		t.Errorf("Depth mismatch: expected 1, got %d", q.Depth())
	}
}

func TestQueueEnqueueNil(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(100, ingestMetrics)

	err := q.Enqueue(nil)
	if err == nil {
		t.Fatal("Expected error for nil signal, got nil")
	}
}

func TestQueueFull(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	capacity := 10
	q := NewQueue(capacity, ingestMetrics)

	// Fill the queue to capacity
	for i := 0; i < capacity; i++ {
		sig := &v1.ArgusSignal{
			SignalId: "test-" + string(rune(i)),
		}
		err := q.Enqueue(sig)
		if err != nil {
			t.Fatalf("Enqueue failed at index %d: %v", i, err)
		}
	}

	// Next enqueue should fail with ErrQueueFull
	err := q.Enqueue(&v1.ArgusSignal{SignalId: "overflow"})
	if err != ErrQueueFull {
		t.Fatalf("Expected ErrQueueFull, got %v", err)
	}

	accepted, dropped := q.Stats()
	if accepted != int64(capacity) || dropped != 1 {
		t.Errorf("Stats mismatch: expected (%d, 1), got (%d, %d)", capacity, accepted, dropped)
	}
}

func TestQueueDequeue(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(100, ingestMetrics)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-1",
		TraceId:  "test-trace-1",
	}

	q.Enqueue(sig)

	ctx := context.Background()
	dequeuedSig, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if dequeuedSig.SignalId != sig.SignalId {
		t.Errorf("Signal ID mismatch: expected %s, got %s", sig.SignalId, dequeuedSig.SignalId)
	}

	if q.Depth() != 0 {
		t.Errorf("Depth mismatch after dequeue: expected 0, got %d", q.Depth())
	}
}

func TestQueueDequeueTimeout(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(100, ingestMetrics)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestQueueConcurrent(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(10000, ingestMetrics)

	numGoroutines := 100
	numSignalsPerGoroutine := 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	// Enqueue goroutines
	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numSignalsPerGoroutine; i++ {
				sig := &v1.ArgusSignal{
					SignalId: "sig-" + string(rune(id)) + "-" + string(rune(i)),
				}
				if err := q.Enqueue(sig); err == nil {
					successCount.Add(1)
				}
			}
		}(g)
	}

	// Dequeue goroutines
	for g := numGoroutines / 2; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				_, err := q.Dequeue(ctx)
				if err != nil {
					return
				}
			}
		}()
	}

	wg.Wait()

	// Verify that signals were enqueued successfully
	accepted, _ := q.Stats()
	if accepted < int64(numGoroutines/2*numSignalsPerGoroutine/2) {
		t.Logf("Warning: Lower than expected success rate. Accepted: %d", accepted)
	}
}

func TestQueueDepth(t *testing.T) {
	reg := &mockRegisterer{}
	ingestMetrics := metrics.NewIngest(reg)
	q := NewQueue(100, ingestMetrics)

	for i := 0; i < 10; i++ {
		sig := &v1.ArgusSignal{SignalId: "sig-" + string(rune(i))}
		q.Enqueue(sig)
	}

	depth := q.Depth()
	if depth != 10 {
		t.Errorf("Depth mismatch: expected 10, got %d", depth)
	}

	// Remove 5 signals
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		q.Dequeue(ctx)
	}

	depth = q.Depth()
	if depth != 5 {
		t.Errorf("Depth after dequeue mismatch: expected 5, got %d", depth)
	}
}

// Mock registerer for testing
type mockRegisterer struct{}

func (m *mockRegisterer) Register(interface{}) error {
	return nil
}

func (m *mockRegisterer) MustRegister(...interface{}) {
}

func (m *mockRegisterer) Unregister(interface{}) bool {
	return true
}
