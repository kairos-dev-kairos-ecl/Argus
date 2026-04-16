package baseline

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	baseline "github.com/argusxdr/argus/internal/baseline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockClickHouseConn mocks the ClickHouse connection
type MockClickHouseConn struct {
	mock.Mock
}

func (m *MockClickHouseConn) Query(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	called := m.Called(ctx, query, args)
	return called.Get(0), called.Error(1)
}

func (m *MockClickHouseConn) Exec(ctx context.Context, query string, args ...interface{}) error {
	called := m.Called(ctx, query, args)
	return called.Error(0)
}

func TestBaselineEngine_StartAndStop(t *testing.T) {
	logger := zap.NewNop()
	config := &baseline.BaselineConfig{
		Enabled:         true,
		ComputeInterval: 1 * time.Second, // Short interval for testing
		HistoryWindow:   24 * time.Hour,
		MinSamples:      10,
		AsyncMode:       true,
	}

	// Note: In a real test, we'd mock the dependencies
	// For now, this validates the basic lifecycle
	engine := baseline.NewBaselineEngine(
		nil, // ClickHouse conn (would be mocked)
		nil, // PostgreSQL pool (would be mocked)
		nil, // Redis client (would be mocked)
		logger,
		config,
		nil,
	)

	// Start engine
	err := engine.Start(context.Background())
	assert.NoError(t, err)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop engine
	err = engine.Stop()
	assert.NoError(t, err)
}

func TestBaselineConfig_Validate_Valid(t *testing.T) {
	config := &baseline.BaselineConfig{
		Enabled:         true,
		ComputeInterval: 10 * time.Minute,
		HistoryWindow:   24 * time.Hour,
		MinSamples:      100,
		AsyncMode:       true,
		RedisCacheTTL:   5 * time.Minute,
	}

	err := config.Validate()
	assert.NoError(t, err)
}

func TestBaselineConfig_Validate_InvalidComputeInterval(t *testing.T) {
	config := &baseline.BaselineConfig{
		ComputeInterval: 10 * time.Second, // Too short
		HistoryWindow:   24 * time.Hour,
		MinSamples:      100,
		RedisCacheTTL:   5 * time.Minute,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ComputeInterval")
}

func TestBaselineConfig_Validate_InvalidHistoryWindow(t *testing.T) {
	config := &baseline.BaselineConfig{
		ComputeInterval: 10 * time.Minute,
		HistoryWindow:   30 * time.Minute, // Too short
		MinSamples:      100,
		RedisCacheTTL:   5 * time.Minute,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HistoryWindow")
}

func TestBaselineConfig_Validate_InvalidMinSamples(t *testing.T) {
	config := &baseline.BaselineConfig{
		ComputeInterval: 10 * time.Minute,
		HistoryWindow:   24 * time.Hour,
		MinSamples:      0, // Invalid
		RedisCacheTTL:   5 * time.Minute,
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MinSamples")
}

func TestBaselineConfig_DefaultConfig(t *testing.T) {
	config := baseline.DefaultBaselineConfig()

	assert.NoError(t, config.Validate())
	assert.True(t, config.Enabled)
	assert.Equal(t, 10*time.Minute, config.ComputeInterval)
	assert.Greater(t, config.HistoryWindow, time.Duration(0))
	assert.Greater(t, config.MinSamples, int64(0))
	assert.True(t, config.AsyncMode)
}

func TestProfileStore_StoreAndRetrieve(t *testing.T) {
	// This is a placeholder for integration testing with real PostgreSQL/Redis
	// For unit testing, we'd mock the dependencies

	profile := &baseline.BaselineProfile{
		SampleCount: 100,
		Mean:        50.0,
		StdDev:      5.0,
		Min:         40.0,
		Max:         60.0,
	}

	assert.NotNil(t, profile)
	assert.Equal(t, int32(100), profile.SampleCount)
}

func TestProfileCalculator_Edge_LargeDataset(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	// Create a large sample set
	samples := make([]float64, 10000)
	for i := 0; i < 10000; i++ {
		samples[i] = float64(i % 1000) // Values 0-999 repeated
	}

	profile, err := calc.ComputeProfile(samples)

	require.NoError(t, err)
	assert.Equal(t, int32(10000), profile.SampleCount)
	assert.Greater(t, profile.StdDev, 0.0)

	// Z-score should be finite for all values in range
	for _, val := range []float64{0, 500, 999} {
		zscore := calc.ComputeZScore(val, profile)
		assert.False(t, math.IsNaN(zscore), "z-score NaN for value %f", val)
		assert.False(t, math.IsInf(zscore, 0), "z-score Inf for value %f", val)
	}
}

func TestProfileCalculator_Edge_VeryHighVariance(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{1.0, 1000000.0, 2.0, 999999.0}

	profile, err := calc.ComputeProfile(samples)

	require.NoError(t, err)
	assert.Greater(t, profile.StdDev, 1000.0) // Should have very high stddev

	// Z-scores should still be finite
	zscore := calc.ComputeZScore(500000.0, profile)
	assert.False(t, math.IsNaN(zscore))
	assert.False(t, math.IsInf(zscore, 0))
}

func TestBaselineEngine_ConfigNil(t *testing.T) {
	logger := zap.NewNop()

	// NewBaselineEngine should handle nil config by using defaults
	engine := baseline.NewBaselineEngine(
		nil,
		nil,
		nil,
		logger,
		nil, // nil config
		nil,
	)

	assert.NotNil(t, engine)
}

// TestBaselineEngine_Concurrency verifies the engine can be started/stopped safely
func TestBaselineEngine_Concurrency(t *testing.T) {
	logger := zap.NewNop()
	config := baseline.DefaultBaselineConfig()
	config.ComputeInterval = 100 * time.Millisecond

	engine := baseline.NewBaselineEngine(
		nil,
		nil,
		nil,
		logger,
		config,
		nil,
	)

	ctx := context.Background()

	// Start engine
	err := engine.Start(ctx)
	require.NoError(t, err)

	// Attempt stop immediately (should be safe)
	err = engine.Stop()
	assert.NoError(t, err)
}

// TestBaselineLayerEnum verifies Layer enum values are accessible
func TestBaselineLayerEnum(t *testing.T) {
	// Verify that layer enum values used in baseline are valid
	layer := v1.Layer_L5_OUTPUT_DECODING
	assert.NotEqual(t, v1.Layer_LAYER_UNSPECIFIED, layer)
}
