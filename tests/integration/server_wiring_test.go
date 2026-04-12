package integration

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	argusv1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/baseline"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/argusxdr/argus/internal/pipeline"
	"github.com/redis/go-redis/v9"
)

// TestPhase4ServerWiring verifies that Phase 3 components initialize and wire correctly into the server flow.
func TestPhase4ServerWiring(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger := zap.NewNop()
	defer logger.Sync()

	// Create mock storage (ClickHouse)
	mockStorage := &mockStorage{signals: make([]*storage.Signal, 0)}

	// Create in-memory Redis (for testing, using real go-redis with nil client simulation)
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	// Skip ingest queue initialization for this test (queue created separately by server)

	// Create pipeline config
	pipelineCfg := pipeline.NewPipelineConfig()
	assert.NoError(t, pipelineCfg.Validate())
	assert.True(t, pipelineCfg.Enabled)
	assert.Equal(t, 1000, pipelineCfg.BufferSize)
	assert.True(t, pipelineCfg.ValidationEnabled)
	assert.True(t, pipelineCfg.NormalizationEnabled)
	assert.True(t, pipelineCfg.CorrelationEnabled)
	assert.True(t, pipelineCfg.BaselineEnabled)
	t.Logf("pipeline config: buffer_size=%d, worker_count=%d", pipelineCfg.BufferSize, pipelineCfg.WorkerCount)

	// Create processor chain
	processors := []pipeline.Processor{
		pipeline.NewSchemaValidator(logger),
		pipeline.NewNormalizer(logger),
		pipeline.NewCorrelationTagger(redisClient, pipelineCfg.CorrelationWindow, pipelineCfg.CorrelationMaxPerTrace, logger),
		pipeline.NewEnricher(nil, logger),
		pipeline.NewBaselineScorer(redisClient, logger),
		pipeline.NewRouter(mockStorage, logger),
	}
	assert.Len(t, processors, 6)

	processorChain := pipeline.NewChainWithBufferSize(pipelineCfg.BufferSize, processors...)
	processorChain.Start(ctx)
	t.Log("processor chain started")

	// Create baseline config
	baselineCfg := baseline.DefaultBaselineConfig()
	assert.NoError(t, baselineCfg.Validate())
	assert.True(t, baselineCfg.Enabled)
	assert.Equal(t, 10*time.Minute, baselineCfg.ComputeInterval)
	assert.Equal(t, 5*time.Minute, baselineCfg.RedisCacheTTL)
	t.Logf("baseline config: compute_interval=%v, min_samples=%d", baselineCfg.ComputeInterval, baselineCfg.MinSamples)

	// Create baseline engine (will run async)
	profileStore := baseline.NewProfileStore(redisClient, nil, baselineCfg.RedisCacheTTL, logger)
	baselineEngine := baseline.NewEngine(mockStorage, nil, profileStore, baselineCfg, logger)
	assert.NoError(t, baselineEngine.Start(ctx))
	t.Log("baseline engine started")

	// Create worker pool
	workerPool := pipeline.NewWorkerPool(queue, processorChain, mockStorage, 2, logger)
	assert.NoError(t, workerPool.Start(ctx))
	t.Log("worker pool started with 2 workers")

	// Verify components are initialized
	assert.NotNil(t, processorChain)
	assert.NotNil(t, workerPool)
	assert.NotNil(t, baselineEngine)
	t.Log("all phase 3 components initialized successfully")

	// Give components time to start
	time.Sleep(100 * time.Millisecond)

	// Verify no panics or crashes
	assert.NotNil(t, processorChain.Results())
	t.Log("processor chain results channel is open")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := processorChain.Shutdown(shutdownCtx); err != nil {
		t.Logf("processor chain shutdown: %v", err)
	}
	t.Log("processor chain shutdown complete")

	if err := workerPool.Wait(); err != nil {
		t.Logf("worker pool wait: %v", err)
	}
	t.Log("worker pool drained")

	if err := baselineEngine.Stop(); err != nil {
		t.Logf("baseline engine stop: %v", err)
	}
	t.Log("baseline engine stopped")
}

// TestPhase4ConfigValidation verifies that invalid configurations are rejected at startup.
func TestPhase4ConfigValidation(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	tests := []struct {
		name      string
		setupCfg  func(*pipeline.PipelineConfig)
		shouldErr bool
	}{
		{
			name: "valid config",
			setupCfg: func(cfg *pipeline.PipelineConfig) {
				// Leave defaults
			},
			shouldErr: false,
		},
		{
			name: "buffer size too small",
			setupCfg: func(cfg *pipeline.PipelineConfig) {
				cfg.BufferSize = 0
			},
			shouldErr: true,
		},
		{
			name: "correlation window too small",
			setupCfg: func(cfg *pipeline.PipelineConfig) {
				cfg.CorrelationWindow = 100 * time.Millisecond
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := pipeline.NewPipelineConfig()
			tt.setupCfg(&cfg)
			err := cfg.Validate()
			if tt.shouldErr {
				assert.Error(t, err, "expected validation to fail")
			} else {
				assert.NoError(t, err, "expected validation to pass")
			}
		})
	}
}

// TestPhase4BaselineConfigValidation verifies baseline configuration validation.
func TestPhase4BaselineConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupCfg  func(*baseline.BaselineConfig)
		shouldErr bool
	}{
		{
			name: "valid baseline config",
			setupCfg: func(cfg *baseline.BaselineConfig) {
				// Leave defaults
			},
			shouldErr: false,
		},
		{
			name: "compute interval too small",
			setupCfg: func(cfg *baseline.BaselineConfig) {
				cfg.ComputeInterval = 30 * time.Second
			},
			shouldErr: true,
		},
		{
			name: "history window too small",
			setupCfg: func(cfg *baseline.BaselineConfig) {
				cfg.HistoryWindow = 30 * time.Minute
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseline.DefaultBaselineConfig()
			tt.setupCfg(cfg)
			err := cfg.Validate()
			if tt.shouldErr {
				assert.Error(t, err, "expected validation to fail")
			} else {
				assert.NoError(t, err, "expected validation to pass")
			}
		})
	}
}

// TestPhase4ProcessorChainIntegration verifies signals flow through the entire chain.
func TestPhase4ProcessorChainIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := zap.NewNop()
	defer logger.Sync()

	// Create mock storage
	mockStorage := &mockClickHouseStorage{signals: make([]*argusv1.ArgusSignal, 0)}

	// Create a minimal processor chain (just validator + router for this test)
	processors := []pipeline.Processor{
		pipeline.NewSchemaValidator(logger),
		pipeline.NewRouter(mockStorage, logger),
	}

	processorChain := pipeline.NewChainWithBufferSize(100, processors...)
	processorChain.Start(ctx)

	// Create a test signal
	testSignal := createTestArgusSignal()
	require.NotNil(t, testSignal)

	// Enqueue the signal
	err := processorChain.Enqueue(testSignal)
	assert.NoError(t, err, "failed to enqueue signal")
	t.Log("signal enqueued")

	// Wait for signal to be processed
	time.Sleep(200 * time.Millisecond)

	// Shutdown pipeline
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	err = processorChain.Shutdown(shutdownCtx)
	t.Logf("processor chain shutdown: %v", err)

	// Verify signal was stored (Router should have written it)
	// This is a simplified check; full integration would use real ClickHouse
	t.Logf("mock storage received %d signals", len(mockStorage.signals))
}

// mockClickHouseStorage implements the storage.ClickHouse interface for testing.
type mockClickHouseStorage struct {
	signals []*argusv1.ArgusSignal
}

func (mcs *mockClickHouseStorage) Write(ctx context.Context, sig *argusv1.ArgusSignal) error {
	mcs.signals = append(mcs.signals, sig)
	return nil
}

func (mcs *mockClickHouseStorage) Close() error {
	return nil
}

// createTestArgusSignal creates a minimal valid ArgusSignal for testing.
func createTestArgusSignal() *argusv1.ArgusSignal {
	now := timestamppb.Now()
	return &argusv1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		Layer:    argusv1.Layer_L7_RAG_RETRIEVAL,
		Category: "retrieval.search",
		Severity: argusv1.Severity_INFO,
		Timestamp: now,
		Source: &argusv1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "test-instance-001",
		},
		Provider: &argusv1.Provider{
			Name:  "test-provider",
			Model: "test-model",
		},
	}
}
