package pipeline_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/argusxdr/argus/internal/baseline"
	"github.com/argusxdr/argus/internal/pipeline"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// TestBaselineScorer_ProcessorInterfaceCompliance tests that BaselineScorer implements the Processor interface
func TestBaselineScorer_ProcessorInterfaceCompliance(t *testing.T) {
	logger := zap.NewNop()

	// Create scorer with nil Redis (will pass signals through)
	scorer := pipeline.NewBaselineScorer(nil, logger)

	// Verify Name() method
	name := scorer.Name()
	assert.Equal(t, "BaselineScorer", name)

	// Verify Process() method signature
	ctx := context.Background()
	sig := &v1.ArgusSignal{
		SignalId: "test",
		Source:   &v1.Source{AppId: "test"},
		Layer:    v1.Layer_L4_TRANSFORMER,
		Category: "test",
		Timestamp: timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := scorer.Process(ctx, sig)
	// Should not panic and should return valid types
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig.SignalId, result.SignalId)
}

// TestBaselineScorer_NoMetricExtractable_PassesThrough tests signals without metrics
func TestBaselineScorer_NoMetricExtractable_PassesThrough(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	scorer := pipeline.NewBaselineScorer(nil, logger)

	// Signal with no duration_ms and no layer-specific metrics
	sig := &v1.ArgusSignal{
		SignalId: "signal-006",
		Source: &v1.Source{
			AppId: "test-app",
		},
		Layer:      v1.Layer_L1_HARDWARE,
		Category:   "cpu_utilization",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		// No DurationMs, no context
	}

	result, err := scorer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Signal should pass through unchanged
	if result.Enrichment != nil {
		assert.Equal(t, float32(0), result.Enrichment.BaselineDeviation)
	}
}

// TestBaselineScorer_NilSignal_ReturnsNil tests that nil signals are handled
func TestBaselineScorer_NilSignal_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	scorer := pipeline.NewBaselineScorer(nil, logger)

	result, err := scorer.Process(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// TestBaselineScorer_AlreadyEnrichedSignal_PassesThrough tests that already-enriched signals are returned unchanged
func TestBaselineScorer_AlreadyEnrichedSignal_PassesThrough(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	scorer := pipeline.NewBaselineScorer(nil, logger)

	sig := &v1.ArgusSignal{
		SignalId: "signal-005",
		Source: &v1.Source{
			AppId: "test-app",
		},
		Layer:      v1.Layer_L4_TRANSFORMER,
		Category:   "generation",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		DurationMs: ptrFloat32(60.0),
		Enrichment: &v1.Enrichment{
			BaselineDeviation: ptrFloat32(2.5), // Already enriched
		},
	}

	result, err := scorer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return the same z-score (unchanged)
	require.NotNil(t, result.Enrichment.BaselineDeviation)
	assert.Equal(t, float32(2.5), *result.Enrichment.BaselineDeviation)
}

// TestBaselineScorer_RedisUnavailable_PassesThrough tests non-fatal Redis error handling
func TestBaselineScorer_RedisUnavailable_PassesThrough(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create scorer with nil Redis (simulates unavailable Redis)
	scorer := pipeline.NewBaselineScorer(nil, logger)

	sig := &v1.ArgusSignal{
		SignalId: "signal-004",
		Source: &v1.Source{
			AppId: "test-app",
		},
		Layer:      v1.Layer_L4_TRANSFORMER,
		Category:   "generation",
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		DurationMs: ptrFloat32(60.0),
	}

	// Process should succeed despite Redis being nil (non-fatal)
	result, err := scorer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Signal should be returned unchanged
	if result.Enrichment != nil {
		assert.Equal(t, float32(0), result.Enrichment.BaselineDeviation)
	}
}

// TestComputeZScore_Helper tests the ComputeZScore helper function
func TestComputeZScore_Helper(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		profile *baseline.BaselineProfile
		expected float64
	}{
		{
			name:  "normal case",
			value: 60.0,
			profile: &baseline.BaselineProfile{
				Mean:   50.0,
				StdDev: 10.0,
			},
			expected: 1.0, // (60 - 50) / 10 = 1.0
		},
		{
			name:  "zero stddev",
			value: 60.0,
			profile: &baseline.BaselineProfile{
				Mean:   50.0,
				StdDev: 0.0, // Should be clamped to 0
			},
			expected: 0.0,
		},
		{
			name:     "nil profile",
			value:    60.0,
			profile:  nil,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := baseline.ComputeZScore(tt.value, tt.profile)
			assert.InDelta(t, tt.expected, result, 0.001,
				"ComputeZScore(%f, %+v) = %f, want %f", tt.value, tt.profile, result, tt.expected)
		})
	}
}

// ptrFloat32 is a helper to create float32 pointers
func ptrFloat32(v float32) *float32 {
	return &v
}
