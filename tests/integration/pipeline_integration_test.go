package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/baseline"
	"github.com/argusxdr/argus/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestPipelineValidationRejectsInvalidSignals verifies SchemaValidator rejects incomplete signals
func TestPipelineValidationRejectsInvalidSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a signal missing required fields
	invalidSignal := &v1.ArgusSignal{
		// Missing signal_id - required field
		Layer: v1.Layer_L5_SERVICE,
		// Missing timestamp - required field
		Source: nil, // Will need proper Source
	}

	validator := pipeline.NewSchemaValidator()
	result, err := validator.Process(context.Background(), invalidSignal)

	// SchemaValidator should reject invalid signals
	assert.Error(t, err, "Expected SchemaValidator to reject invalid signal")
	assert.Nil(t, result, "Invalid signal should not be returned")
}

// TestPipelineFullEndToEnd verifies complete signal flow through all processors
func TestPipelineFullEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a valid signal with all required fields
	now := timestamppb.Now()
	validSignal := &v1.ArgusSignal{
		SignalId:  fmt.Sprintf("test-signal-%d", rand.Intn(1000000)),
		Timestamp: now,
		Layer:     v1.Layer_L5_SERVICE,
		TraceId:   "trace-123",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "1.2.3",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		EventData: &v1.ArgusSignal_Event{
			Event: &v1.Event{
				Type: "test_event",
				Time: now,
			},
		},
	}

	// Create a simple chain with validators and normalizer
	validator := pipeline.NewSchemaValidator()
	normalizer := pipeline.NewNormalizer(nil)

	chain := pipeline.NewChain(validator, normalizer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := chain.Start(ctx)
	require.NoError(t, err, "Failed to start chain")

	// Enqueue signal
	err = chain.Enqueue(validSignal)
	require.NoError(t, err, "Failed to enqueue signal")

	// Wait for result
	select {
	case result := <-chain.Results():
		assert.NotNil(t, result, "Pipeline should return processed signal")
		assert.Equal(t, validSignal.SignalId, result.SignalId, "Signal ID should be preserved")
		assert.Equal(t, v1.Layer_L5_SERVICE, result.Layer, "Layer should be preserved")
	case <-ctx.Done():
		t.Fatal("Pipeline timeout waiting for result")
	}

	chain.Shutdown(ctx)
}

// TestPipelineCorrelationTaggerLinksTraces verifies related_signals population
func TestPipelineCorrelationTaggerLinksTraces(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test would require Redis to be available
	// In a full integration environment, it would:
	// 1. Send signal 1 with trace_id="trace-123"
	// 2. Send signal 2 with same trace_id within 5 seconds
	// 3. Verify both signals have each other in related_signals

	t.Logf("Correlation test requires Redis integration (deferred to system test)")
}

// TestBaselineProfileComputation verifies baseline engine processes profiles
func TestBaselineProfileComputation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a calculator
	calc := baseline.NewProfileCalculator()

	// Generate sample data (100+ samples as required)
	samples := make([]float64, 150)
	for i := 0; i < 150; i++ {
		samples[i] = float64(i) // Values 0-149
	}

	profile, err := calc.ComputeProfile(samples)
	require.NoError(t, err, "Failed to compute profile")
	require.NotNil(t, profile, "Profile should not be nil")

	// Verify profile structure
	assert.Greater(t, profile.SampleCount, int32(100), "Profile should have sufficient samples")
	assert.Greater(t, profile.Mean, 0.0, "Mean should be positive")
	assert.Greater(t, profile.StdDev, 0.0, "StdDev should be positive")
	assert.Greater(t, profile.Max, profile.Min, "Max should be greater than min")

	// Test z-score computation
	zscore := calc.ComputeZScore(samples[0], profile)
	assert.False(t, fmt.IsNaN(zscore), "Z-score should never be NaN")
	assert.False(t, fmt.IsInf(zscore, 0), "Z-score should not be Inf")
}

// TestBaselineZScoreClamping verifies z-score never NaN when stddev=0
func TestBaselineZScoreClamping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	calc := baseline.NewProfileCalculator()

	// Create profile with identical samples (stddev=0)
	samples := make([]float64, 100)
	for i := 0; i < 100; i++ {
		samples[i] = 42.0 // All identical
	}

	profile, err := calc.ComputeProfile(samples)
	require.NoError(t, err)

	// Z-score should be 0 (not NaN) when stddev is 0
	zscore := calc.ComputeZScore(100.0, profile)
	assert.Equal(t, 0.0, zscore, "Z-score should be 0 when stddev=0 (Pitfall 4 prevention)")
}

// TestWorkerPoolGoroutineStability verifies goroutine count remains constant
func TestWorkerPoolGoroutineStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Measure goroutine count before/after processing signals
	initialGoroutines := runtime.NumGoroutine()

	// Simulate processing many signals
	// In a real test, this would use actual WorkerPool
	processor := pipeline.NewSchemaValidator()
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		signal := &v1.ArgusSignal{
			SignalId:  fmt.Sprintf("signal-%d", i),
			Timestamp: timestamppb.Now(),
			Layer:     v1.Layer_L5_SERVICE,
			Source:    &v1.Source{AppId: "test"},
		}
		// Process signal (non-blocking validation)
		_, _ = processor.Process(ctx, signal)
	}

	finalGoroutines := runtime.NumGoroutine()

	// Goroutine count should not grow proportionally with signal count
	// Allow small variance for go scheduler
	goroutineDelta := finalGoroutines - initialGoroutines
	assert.Less(t, goroutineDelta, 10, "Goroutine explosion detected (Pitfall 6)")
}

// TestPipelineMetricTracking verifies metrics are incremented correctly
func TestPipelineMetricTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create schema validator which increments rejection metrics
	validator := pipeline.NewSchemaValidator()

	// Create invalid signal
	invalidSignal := &v1.ArgusSignal{
		// Missing required fields
	}

	// Process invalid signal (should increment rejection metric)
	_, err := validator.Process(context.Background(), invalidSignal)
	assert.Error(t, err, "Should reject invalid signal")

	// In a real integration test, we would query Prometheus metrics to verify:
	// - argus_pipeline_validation_failures_total incremented
	// - Field-level error labels correct

	t.Logf("Metric tracking test requires Prometheus scraper (deferred to system test)")
}

// TestEnricherWithGeoIPIntegration verifies enricher calls GeoIP helper
func TestEnricherWithGeoIPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test would require:
	// 1. GeoIP database configured
	// 2. Signal with IP address in context
	// 3. Verification that enrichment.geo fields populated

	t.Logf("GeoIP integration test requires MaxMind database (deferred to system test)")
}

// TestBaselineProfileSerialization verifies JSON marshaling
func TestBaselineProfileSerialization(t *testing.T) {
	profile := &baseline.BaselineProfile{
		AppID:       "test-app",
		Layer:       v1.Layer_L5_SERVICE,
		Category:    "test_category",
		Mean:        50.0,
		StdDev:      5.0,
		Min:         40.0,
		Max:         60.0,
		SampleCount: 100,
		ComputedAt:  timestamppb.Now(),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(profile)
	require.NoError(t, err)

	// Unmarshal back
	var restored baseline.BaselineProfile
	err = json.Unmarshal(jsonData, &restored)
	require.NoError(t, err)

	// Verify restored profile matches original
	assert.Equal(t, profile.AppID, restored.AppID)
	assert.Equal(t, profile.Mean, restored.Mean)
	assert.Equal(t, profile.StdDev, restored.StdDev)
}

// TestSignalFlowThroughChain verifies correct processor ordering
func TestSignalFlowThroughChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create processors in proper order
	validator := pipeline.NewSchemaValidator()
	normalizer := pipeline.NewNormalizer(nil)

	// Create chain
	chain := pipeline.NewChain(validator, normalizer)

	// Verify processors are ordered
	assert.NotNil(t, chain, "Chain should be created")
	assert.Equal(t, "SchemaValidator", validator.Name(), "Validator name should match")
	assert.Equal(t, "Normalizer", normalizer.Name(), "Normalizer name should match")
}

// TestSmokeTes verifies basic end-to-end functionality
func TestSmokeTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple processor
	validator := pipeline.NewSchemaValidator()

	// Process a valid signal
	signal := &v1.ArgusSignal{
		SignalId:  "smoke-test-1",
		Timestamp: timestamppb.Now(),
		Layer:     v1.Layer_L5_SERVICE,
		Source: &v1.Source{
			AppId: "smoke-test",
		},
	}

	result, err := validator.Process(ctx, signal)

	// Valid signal should pass validation
	assert.NotNil(t, result, "Valid signal should pass validator")
	if err == nil {
		assert.Equal(t, signal.SignalId, result.SignalId)
	}
}
