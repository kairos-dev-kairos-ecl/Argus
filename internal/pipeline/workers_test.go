package pipeline

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWriter records Write calls and the time each call was made.
type mockWriter struct {
	callCount int64
	callTime  atomic.Value // stores time.Time of the last Write
}

func (m *mockWriter) Write(_ context.Context, sig *v1.ArgusSignal) error {
	atomic.AddInt64(&m.callCount, 1)
	m.callTime.Store(time.Now())
	return nil
}

// mockEnqueuer records Enqueue calls and the time each call was made.
type mockEnqueuer struct {
	callCount int64
	callTime  atomic.Value // stores time.Time of the last Enqueue
}

func (m *mockEnqueuer) Enqueue(_ *v1.ArgusSignal) {
	atomic.AddInt64(&m.callCount, 1)
	m.callTime.Store(time.Now())
}

// passThruProcessor passes every signal through unchanged.
type passThruProcessor struct{}

func (p *passThruProcessor) Process(_ context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	return sig, nil
}

func (p *passThruProcessor) Name() string { return "pass-thru" }

// TestWorkerPool_DispatchesToAsyncEnqueuerAfterWrite verifies:
//  1. Write is called before Enqueue.
//  2. Both Write and Enqueue are called exactly once per signal.
func TestWorkerPool_DispatchesToAsyncEnqueuerAfterWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build a trivial pipeline chain with a pass-through processor.
	chain := NewChain(&passThruProcessor{})
	chain.Start(ctx)

	// Input channel for the WorkerPool.
	ingestCh := make(chan *v1.ArgusSignal, 1)

	writer := &mockWriter{}
	enqueuer := &mockEnqueuer{}

	wp := NewWorkerPool(ingestCh, chain, writer, 1, nil)
	wp.SetAsyncEnqueuer(enqueuer)
	require.NoError(t, wp.Start(ctx))

	// Send a signal through the pool.
	sig := &v1.ArgusSignal{SignalId: "test-signal-001", TraceId: "trace-001"}
	ingestCh <- sig

	// Wait for both Write and Enqueue to be called.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&writer.callCount) >= 1 && atomic.LoadInt64(&enqueuer.callCount) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&writer.callCount), "Write should be called exactly once")
	assert.Equal(t, int64(1), atomic.LoadInt64(&enqueuer.callCount), "Enqueue should be called exactly once")

	// Verify ordering: Write must happen before Enqueue.
	wt, wOk := writer.callTime.Load().(time.Time)
	et, eOk := enqueuer.callTime.Load().(time.Time)
	require.True(t, wOk, "writer call time should be recorded")
	require.True(t, eOk, "enqueuer call time should be recorded")
	assert.True(t, !wt.After(et), "Write must happen before or at the same time as Enqueue")
}

// TestWorkerPool_NoEnqueueWhenEnqueuerIsNil verifies that a nil asyncEnqueuer
// does not cause a panic and signals are written to storage normally.
func TestWorkerPool_NoEnqueueWhenEnqueuerIsNil(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain := NewChain(&passThruProcessor{})
	chain.Start(ctx)

	ingestCh := make(chan *v1.ArgusSignal, 1)
	writer := &mockWriter{}

	wp := NewWorkerPool(ingestCh, chain, writer, 1, nil)
	// Do NOT call SetAsyncEnqueuer — asyncEnqueuer is nil.
	require.NoError(t, wp.Start(ctx))

	ingestCh <- &v1.ArgusSignal{SignalId: "test-signal-nil-enqueuer", TraceId: "trace-002"}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&writer.callCount) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&writer.callCount), "Write should be called once even with nil enqueuer")
}
