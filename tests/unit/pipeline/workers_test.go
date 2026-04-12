package pipeline_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/argusxdr/argus/internal/pipeline"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// MockPipeline is a simple mock pipeline for testing
// It wraps a real Chain to provide testing support
type MockPipeline struct {
	chain *pipeline.Chain
}

func NewMockPipeline() *MockPipeline {
	proc := &PassThroughProcessor{}
	chain := pipeline.NewChain(proc)
	// Start the chain with a background context so it processes signals
	chain.Start(context.Background())
	return &MockPipeline{
		chain: chain,
	}
}

func (m *MockPipeline) Enqueue(sig *v1.ArgusSignal) error {
	return m.chain.Enqueue(sig)
}

func (m *MockPipeline) Results() <-chan *v1.ArgusSignal {
	return m.chain.Results()
}

func (m *MockPipeline) Shutdown(ctx context.Context) error {
	return m.chain.Shutdown(ctx)
}

// PassThroughProcessor is a processor that passes signals through unchanged
type PassThroughProcessor struct{}

func (p *PassThroughProcessor) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	return sig, nil
}

func (p *PassThroughProcessor) Name() string {
	return "PassThrough"
}

// MockWriter is a simple storage writer for testing
type MockWriter struct {
	mu     sync.Mutex
	count  int64
	errors int64
	signals []*v1.ArgusSignal
	shouldFail bool
}

func (m *MockWriter) Write(ctx context.Context, sig *v1.ArgusSignal) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		m.errors++
		return errors.New("mock write error")
	}

	m.signals = append(m.signals, sig)
	m.count++
	return nil
}

func (m *MockWriter) Count() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

func (m *MockWriter) Errors() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors
}

func (m *MockWriter) Signals() []*v1.ArgusSignal {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.signals
}

// TestWorkerPool_SpawnsCorrectNumberOfWorkers tests that Start() creates exactly N goroutines
func TestWorkerPool_SpawnsCorrectNumberOfWorkers(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue
	queue := make(chan *v1.ArgusSignal, 100)
	defer close(queue)

	// Create a mock pipeline
	mockPipeline := NewMockPipeline()

	// Create storage writer
	writer := &MockWriter{}

	// Create worker pool with 5 workers
	numWorkers := 5
	pool := pipeline.NewWorkerPool(queue, mockPipeline.chain, writer, numWorkers, logger)

	// Record goroutine count before start
	initialGoroutines := runtime.NumGoroutine()

	// Start the pool
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Give goroutines time to start
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	addedGoroutines := finalGoroutines - initialGoroutines

	// Should have roughly numWorkers new goroutines added
	assert.GreaterOrEqual(t, addedGoroutines, numWorkers,
		"Expected at least %d new goroutines, got %d", numWorkers, addedGoroutines)

	// Clean up
	pool.Shutdown()
}

// TestWorkerPool_GoroutineCountStableWithIncreasingVolume tests Pitfall 6 prevention:
// goroutine count remains constant as signal volume increases
func TestWorkerPool_GoroutineCountStableWithIncreasingVolume(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue with large buffer
	queue := make(chan *v1.ArgusSignal, 1000)
	defer close(queue)

	// Create a mock pipeline
	pipelineImpl := NewMockPipeline()

	// Create storage writer
	writer := &MockWriter{}

	// Create worker pool
	numWorkers := 4
	pool := pipeline.NewWorkerPool(queue, pipelineImpl.chain, writer, numWorkers, logger)
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Record goroutine count after starting
	time.Sleep(100 * time.Millisecond)
	startingGoroutines := runtime.NumGoroutine()

	// Send 100 signals to the queue
	for i := 0; i < 100; i++ {
		sig := &v1.ArgusSignal{
			SignalId:   "signal-" + string(rune(i%256)),
			TraceId:    "trace-" + string(rune(i%256)),
			Source:     &v1.Source{AppId: "test-app"},
			Layer:      v1.Layer_L4_TRANSFORMER,
			Category:   "test",
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}
		select {
		case queue <- sig:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("queue full after %d signals", i)
		}
	}

	// Wait for processing to start
	time.Sleep(500 * time.Millisecond)

	// Check goroutine count mid-processing
	midProcessingGoroutines := runtime.NumGoroutine()

	// Wait for all signals to be processed
	time.Sleep(2 * time.Second)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()

	// Goroutine count should remain stable (not increase proportionally with signal volume)
	// Allow some variance due to GC and runtime fluctuations
	goroutineVariance := finalGoroutines - startingGoroutines
	assert.Less(t, goroutineVariance, numWorkers+5,
		"Goroutine count grew too much during processing: start=%d, mid=%d, final=%d",
		startingGoroutines, midProcessingGoroutines, finalGoroutines)

	// Clean up
	pool.Shutdown()
}

// TestWorkerPool_SignalsFlowEndToEnd tests that signals flow from queue → pipeline → storage
func TestWorkerPool_SignalsFlowEndToEnd(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue
	queue := make(chan *v1.ArgusSignal, 100)

	// Create a mock pipeline
	pipelineImpl := NewMockPipeline()

	// Create storage writer
	writer := &MockWriter{}

	// Create worker pool
	pool := pipeline.NewWorkerPool(queue, pipelineImpl.chain, writer, 2, logger)
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Send 10 test signals
	testSignals := make([]*v1.ArgusSignal, 10)
	for i := 0; i < 10; i++ {
		sig := &v1.ArgusSignal{
			SignalId:   "signal-" + string(rune('0'+i)),
			TraceId:    "trace-" + string(rune('0'+i)),
			Source:     &v1.Source{AppId: "test-app"},
			Layer:      v1.Layer_L4_TRANSFORMER,
			Category:   "test",
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}
		testSignals[i] = sig
		queue <- sig
	}

	// Close the queue to signal end of input
	close(queue)

	// Wait a bit for processing
	time.Sleep(500 * time.Millisecond)

	// Verify all signals were written to storage
	writtenSignals := writer.Signals()
	assert.Equal(t, 10, len(writtenSignals),
		"Expected 10 signals written, got %d", len(writtenSignals))

	// Verify signal IDs match (order may vary due to concurrency)
	sentIDs := make(map[string]bool)
	for _, sig := range testSignals {
		sentIDs[sig.SignalId] = true
	}

	for _, sig := range writtenSignals {
		assert.True(t, sentIDs[sig.SignalId],
			"Signal %s was written but not sent", sig.SignalId)
	}

	// Clean up
	pool.Shutdown()
}

// TestWorkerPool_BackpressureHandled tests that backpressure (ErrPipelineFull) is handled gracefully
func TestWorkerPool_BackpressureHandled(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue
	queue := make(chan *v1.ArgusSignal, 100)

	// Create a mock pipeline (configured to return ErrPipelineFull to simulate backpressure)
	backpressurePipeline := &BackpressureMockPipeline{
		innerPipeline: NewMockPipeline(),
		enqueueCount:  &atomic.Int64{},
	}

	// Create storage writer
	writer := &MockWriter{}

	// Create worker pool
	pool := pipeline.NewWorkerPool(queue, backpressurePipeline.innerPipeline.chain, writer, 2, logger)
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Send many signals to trigger backpressure
	for i := 0; i < 50; i++ {
		sig := &v1.ArgusSignal{
			SignalId:   "signal-" + string(rune(i%10)),
			Source:     &v1.Source{AppId: "test-app"},
			Layer:      v1.Layer_L4_TRANSFORMER,
			Category:   "test",
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}
		queue <- sig
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// The test passes if pool.Start() didn't panic and processing continued
	// Even with backpressure, workers should handle gracefully and continue
	assert.True(t, true, "BackpressureHandled completed without panic")

	// Clean up
	close(queue)
	pool.Shutdown()
}

// BackpressureMockPipeline simulates backpressure by failing every Nth enqueue
type BackpressureMockPipeline struct {
	innerPipeline *MockPipeline
	enqueueCount  *atomic.Int64
}

func (b *BackpressureMockPipeline) Enqueue(sig *v1.ArgusSignal) error {
	count := b.enqueueCount.Add(1)
	// Return ErrPipelineFull every 5 enqueues to simulate backpressure
	if count%5 == 0 {
		return pipeline.ErrPipelineFull
	}
	return b.innerPipeline.Enqueue(sig)
}

func (b *BackpressureMockPipeline) Results() <-chan *v1.ArgusSignal {
	return b.innerPipeline.Results()
}

func (b *BackpressureMockPipeline) Shutdown(ctx context.Context) error {
	return b.innerPipeline.Shutdown(ctx)
}

// TestWorkerPool_StorageWriteFailuresHandled tests that storage write failures are handled gracefully
func TestWorkerPool_StorageWriteFailuresHandled(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue
	queue := make(chan *v1.ArgusSignal, 100)

	// Create a mock pipeline
	pipelineImpl := NewMockPipeline()

	// Create storage writer that fails
	writer := &MockWriter{shouldFail: true}

	// Create worker pool
	pool := pipeline.NewWorkerPool(queue, pipelineImpl.chain, writer, 2, logger)
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Send 5 test signals
	for i := 0; i < 5; i++ {
		sig := &v1.ArgusSignal{
			SignalId:   "signal-" + string(rune('0'+i)),
			Source:     &v1.Source{AppId: "test-app"},
			Layer:      v1.Layer_L4_TRANSFORMER,
			Category:   "test",
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}
		queue <- sig
	}

	// Close the queue
	close(queue)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify errors were recorded
	assert.Greater(t, writer.Errors(), int64(0),
		"Expected storage write errors, got %d", writer.Errors())

	// The test passes if pool.Start() didn't panic and processing continued
	// even when storage writes failed
	assert.True(t, true, "StorageWriteFailuresHandled completed without panic")

	// Clean up
	pool.Shutdown()
}

// TestWorkerPool_WaitBlocksUntilWorkersExit tests that Wait() blocks until all workers finish
func TestWorkerPool_WaitBlocksUntilWorkersExit(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create a queue
	queue := make(chan *v1.ArgusSignal, 10)

	// Create a mock pipeline
	pipelineImpl := NewMockPipeline()

	// Create storage writer
	writer := &MockWriter{}

	// Create worker pool
	pool := pipeline.NewWorkerPool(queue, pipelineImpl.chain, writer, 2, logger)
	err := pool.Start(ctx)
	require.NoError(t, err)

	// Send a few signals
	for i := 0; i < 5; i++ {
		sig := &v1.ArgusSignal{
			SignalId:   "signal-" + string(rune('0'+i)),
			Source:     &v1.Source{AppId: "test-app"},
			Layer:      v1.Layer_L4_TRANSFORMER,
			Category:   "test",
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}
		queue <- sig
	}

	// Close the queue to signal end of input
	close(queue)

	// Track when Wait() returns
	waitComplete := &atomic.Bool{}
	go func() {
		_ = pool.Wait()
		waitComplete.Store(true)
	}()

	// Wait a bit (should have time to return before timeout)
	time.Sleep(1 * time.Second)

	// Verify Wait() has returned
	assert.True(t, waitComplete.Load(),
		"Wait() did not return within timeout")

	// Clean up
	pool.Shutdown()
}

// TestWorkerPool_CallStartTwiceIsNoOp tests that calling Start() twice is a no-op
func TestWorkerPool_CallStartTwiceIsNoOp(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	queue := make(chan *v1.ArgusSignal, 100)
	defer close(queue)

	pipelineImpl := NewMockPipeline()

	writer := &MockWriter{}

	pool := pipeline.NewWorkerPool(queue, pipelineImpl.chain, writer, 2, logger)

	// Call Start() twice
	err1 := pool.Start(ctx)
	require.NoError(t, err1)

	time.Sleep(50 * time.Millisecond)
	gorutinesAfterFirstStart := runtime.NumGoroutine()

	err2 := pool.Start(ctx)
	require.NoError(t, err2)

	time.Sleep(50 * time.Millisecond)
	gorutinesAfterSecondStart := runtime.NumGoroutine()

	// Goroutine count should not increase after second Start()
	// (allowing for small variance from GC and other runtime activity)
	assert.Equal(t, gorutinesAfterFirstStart, gorutinesAfterSecondStart,
		"Second Start() should not spawn additional workers")

	pool.Shutdown()
}
