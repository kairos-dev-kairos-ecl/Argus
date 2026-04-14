package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// mockProcessor is a test processor that optionally modifies signals or returns errors
type mockProcessor struct {
	name       string
	modifyFunc func(*v1.ArgusSignal) (*v1.ArgusSignal, error)
	callCount  int
}

func (m *mockProcessor) Name() string {
	return m.name
}

func (m *mockProcessor) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	m.callCount++
	if m.modifyFunc == nil {
		return sig, nil
	}
	return m.modifyFunc(sig)
}

// testSignal creates a valid test signal
func testSignal(id string) *v1.ArgusSignal {
	return &v1.ArgusSignal{
		SignalId:   id,
		TraceId:    "trace-" + id,
		Layer:      v1.Layer_L10_APPLICATION,
		Category:   "test",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Source: &v1.Source{
			AppId:      "test-app",
			AppVersion: "1.0.0",
			SdkVersion: "1.0.0",
			Environment: "test",
			InstanceId: "instance-1",
		},
	}
}

// TestChainProcessesSignalsInOrder verifies that processors execute in the correct sequence
func TestChainProcessesSignalsInOrder(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create processors that track execution order
	proc1 := &mockProcessor{
		name: "proc1",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			sig.Category = "proc1"
			return sig, nil
		},
	}
	proc2 := &mockProcessor{
		name: "proc2",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			if sig.Category != "proc1" {
				return nil, errors.New("proc1 did not run before proc2")
			}
			sig.Category = "proc2"
			return sig, nil
		},
	}

	chain := NewChain(proc1, proc2)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain.Start(ctx)

	// Enqueue a signal
	sig := testSignal("test-1")
	err := chain.Enqueue(sig)
	require.NoError(t, err)

	// Close input to signal end of processing
	go func() {
		time.Sleep(100 * time.Millisecond)
		chain.inputCh <- nil // Sentinel? Actually, let's close it properly
		close(chain.inputCh)
	}()

	// Receive result
	result := <-chain.Results()
	require.NotNil(t, result)
	assert.Equal(t, "proc2", result.Category)
	assert.Equal(t, 1, proc1.callCount)
	assert.Equal(t, 1, proc2.callCount)
}

// TestChainHandlesProcessorErrors verifies that errors in one processor don't crash the chain
func TestChainHandlesProcessorErrors(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	proc1 := &mockProcessor{
		name: "proc1",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			if sig.SignalId == "fail-me" {
				return nil, errors.New("intentional error in proc1")
			}
			return sig, nil
		},
	}
	proc2 := &mockProcessor{
		name: "proc2",
	}

	chain := NewChain(proc1, proc2)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain.Start(ctx)

	// Enqueue two signals: one that fails, one that succeeds
	err := chain.Enqueue(testSignal("good-1"))
	require.NoError(t, err)

	fail := testSignal("fail-me")
	err = chain.Enqueue(fail)
	require.NoError(t, err)

	err = chain.Enqueue(testSignal("good-2"))
	require.NoError(t, err)

	// Close input
	close(chain.inputCh)

	// Collect results
	results := []*v1.ArgusSignal{}
	for result := range chain.Results() {
		results = append(results, result)
	}

	// Should have 2 results (the failures are silently skipped)
	assert.Equal(t, 2, len(results))
	assert.Equal(t, "good-1", results[0].SignalId)
	assert.Equal(t, "good-2", results[1].SignalId)
}

// TestChainProcessorCanFilterSignals verifies that processors can filter signals by returning nil
func TestChainProcessorCanFilterSignals(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Processor that filters out signals with category "skip"
	filter := &mockProcessor{
		name: "filter",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			if sig.Category == "skip" {
				return nil, nil // Filter without error
			}
			return sig, nil
		},
	}

	chain := NewChain(filter)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain.Start(ctx)

	// Enqueue signals with different categories
	good := testSignal("good-1")
	chain.Enqueue(good)

	skip := testSignal("skip-1")
	skip.Category = "skip"
	chain.Enqueue(skip)

	good2 := testSignal("good-2")
	chain.Enqueue(good2)

	// Close input and collect results
	close(chain.inputCh)
	results := []*v1.ArgusSignal{}
	for result := range chain.Results() {
		results = append(results, result)
	}

	// Only non-filtered signals should appear
	assert.Equal(t, 2, len(results))
	assert.Equal(t, "good-1", results[0].SignalId)
	assert.Equal(t, "good-2", results[1].SignalId)
}

// TestChainBackpressure verifies that full channels cause Enqueue to return errors
func TestChainBackpressure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create a chain with a small buffer
	chain := NewChainWithBufferSize(2, &mockProcessor{name: "slow"})
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Don't start the processing goroutine, so signals pile up
	// Actually, we need to start it but have it block on the output channel

	// Better approach: create a slow processor
	slowProc := &mockProcessor{
		name: "slow",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			time.Sleep(100 * time.Millisecond)
			return sig, nil
		},
	}
	chain = NewChainWithBufferSize(2, slowProc)
	chain.SetLogger(logger)
	chain.Start(ctx)

	// Enqueue more signals than the buffer can hold
	// The first 2 should succeed, the rest should fail with ErrPipelineFull
	results := []error{}
	for i := 0; i < 10; i++ {
		sig := testSignal("sig-" + string(rune('0'+i)))
		err := chain.Enqueue(sig)
		results = append(results, err)
	}

	// Count successes and failures
	successes := 0
	failures := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrPipelineFull) {
			failures++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Should have some failures due to backpressure
	assert.Greater(t, failures, 0, "expected backpressure to cause ErrPipelineFull")
	assert.Less(t, successes, 10, "expected not all signals to succeed")
}

// TestChainShutdownGracefully verifies graceful shutdown drains pending signals
func TestChainShutdownGracefully(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	procCount := 0
	proc := &mockProcessor{
		name: "counter",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			procCount++
			return sig, nil
		},
	}

	chain := NewChain(proc)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain.Start(ctx)

	// Enqueue several signals
	for i := 0; i < 5; i++ {
		chain.Enqueue(testSignal("sig-" + string(rune('0'+i))))
	}

	// Gracefully shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := chain.Shutdown(shutdownCtx)
	assert.NoError(t, err)
	assert.True(t, chain.shutdown)

	// All signals should have been processed
	assert.Equal(t, 5, procCount)

	// New enqueues should fail
	err = chain.Enqueue(testSignal("should-fail"))
	assert.ErrorIs(t, err, ErrChainShutdown)
}

// TestChainRejectsNilSignals verifies that nil signals are rejected
func TestChainRejectsNilSignals(t *testing.T) {
	chain := NewChain(&mockProcessor{name: "test"})

	err := chain.Enqueue(nil)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "signal cannot be nil")
}

// TestChainEnqueueErrorsWhenFull verifies ErrPipelineFull is returned correctly
func TestChainEnqueueErrorsWhenFull(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create a chain with size 1, don't start the goroutine
	chain := NewChainWithBufferSize(1, &mockProcessor{name: "test"})
	chain.SetLogger(logger)

	// Don't call Start(), so the input channel never drains

	// Enqueue one signal (should succeed, filling the buffer)
	err := chain.Enqueue(testSignal("sig-1"))
	assert.NoError(t, err)

	// Try to enqueue another (should fail because buffer is full)
	err = chain.Enqueue(testSignal("sig-2"))
	assert.ErrorIs(t, err, ErrPipelineFull)
}

// TestChainMultipleProcessors verifies a chain of multiple processors
func TestChainMultipleProcessors(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create a chain of 3 processors that each increment a counter
	p1 := &mockProcessor{name: "p1"}
	p2 := &mockProcessor{name: "p2"}
	p3 := &mockProcessor{name: "p3"}

	chain := NewChain(p1, p2, p3)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chain.Start(ctx)

	// Enqueue a signal
	sig := testSignal("test-1")
	chain.Enqueue(sig)

	// Close input
	close(chain.inputCh)

	// Collect result
	result := <-chain.Results()
	require.NotNil(t, result)

	// All processors should have run exactly once
	assert.Equal(t, 1, p1.callCount)
	assert.Equal(t, 1, p2.callCount)
	assert.Equal(t, 1, p3.callCount)
}

// TestChainContextCancellation verifies that context cancellation stops processing
func TestChainContextCancellation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	procCount := 0
	proc := &mockProcessor{
		name: "counter",
		modifyFunc: func(sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
			procCount++
			time.Sleep(100 * time.Millisecond) // Simulate work
			return sig, nil
		},
	}

	chain := NewChain(proc)
	chain.SetLogger(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	chain.Start(ctx)

	// Enqueue signals rapidly
	for i := 0; i < 20; i++ {
		chain.Enqueue(testSignal("sig-" + string(rune('0'+i%10))))
	}

	// Wait for context to expire
	<-ctx.Done()

	// Due to cancellation, not all signals should have been processed
	assert.Less(t, procCount, 20)
}

// TestConfigValidation verifies PipelineConfig.Validate()
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  PipelineConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  NewPipelineConfig(),
			wantErr: false,
		},
		{
			name: "invalid buffer size",
			config: func() PipelineConfig {
				cfg := NewPipelineConfig()
				cfg.BufferSize = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "buffer_size must be >= 1",
		},
		{
			name: "invalid correlation window",
			config: func() PipelineConfig {
				cfg := NewPipelineConfig()
				cfg.CorrelationWindow = 100 * time.Millisecond
				return cfg
			}(),
			wantErr: true,
			errMsg:  "correlation_window must be >= 1 second",
		},
		{
			name: "invalid baseline interval",
			config: func() PipelineConfig {
				cfg := NewPipelineConfig()
				cfg.BaselineComputeInterval = 30 * time.Second
				return cfg
			}(),
			wantErr: true,
			errMsg:  "baseline_compute_interval must be >= 1 minute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestChainInputOutputDepth verifies depth reporting
func TestChainInputOutputDepth(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	chain := NewChainWithBufferSize(10, &mockProcessor{name: "test"})
	chain.SetLogger(logger)

	// Initially empty
	assert.Equal(t, 0, chain.InputDepth())
	assert.Equal(t, 0, chain.OutputDepth())

	// Add some signals
	for i := 0; i < 3; i++ {
		chain.Enqueue(testSignal("sig-" + string(rune('0'+i))))
	}

	// Should show in input depth
	assert.Equal(t, 3, chain.InputDepth())
}
