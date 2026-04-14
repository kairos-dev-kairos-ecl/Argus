package baseline

import (
	"math"
	"testing"

	baseline "github.com/argusxdr/argus/internal/baseline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileCalculator_ComputeProfile_NormalDistribution(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{10.0, 20.0, 30.0, 40.0, 50.0}

	profile, err := calc.ComputeProfile(samples)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, int32(5), profile.SampleCount)
	assert.Equal(t, 30.0, profile.Mean) // (10+20+30+40+50)/5 = 30
	assert.Greater(t, profile.StdDev, 0.0)
	assert.Equal(t, 10.0, profile.Min)
	assert.Equal(t, 50.0, profile.Max)
}

func TestProfileCalculator_ComputeProfile_SingleSample(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{42.5}

	profile, err := calc.ComputeProfile(samples)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, int32(1), profile.SampleCount)
	assert.Equal(t, 42.5, profile.Mean)
	assert.Equal(t, 0.0, profile.StdDev) // Single point has no variance
	assert.Equal(t, 42.5, profile.Min)
	assert.Equal(t, 42.5, profile.Max)
}

func TestProfileCalculator_ComputeProfile_IdenticalSamples(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{5.0, 5.0, 5.0, 5.0, 5.0}

	profile, err := calc.ComputeProfile(samples)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, int32(5), profile.SampleCount)
	assert.Equal(t, 5.0, profile.Mean)
	assert.Equal(t, 0.0, profile.StdDev) // All same = no variance
	assert.Equal(t, 5.0, profile.Min)
	assert.Equal(t, 5.0, profile.Max)
}

func TestProfileCalculator_ComputeProfile_EmptyInput(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	_, err := calc.ComputeProfile([]float64{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestProfileCalculator_ComputeZScore_Normal(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	profile := &baseline.BaselineProfile{
		SampleCount: 10,
		Mean:        100.0,
		StdDev:      10.0,
		Min:         80.0,
		Max:         120.0,
	}

	// 1 standard deviation above mean
	zscore := calc.ComputeZScore(110.0, profile)
	assert.InDelta(t, 1.0, zscore, 0.001)

	// Exactly at mean
	zscore = calc.ComputeZScore(100.0, profile)
	assert.InDelta(t, 0.0, zscore, 0.001)

	// 2 standard deviations below mean
	zscore = calc.ComputeZScore(80.0, profile)
	assert.InDelta(t, -2.0, zscore, 0.001)
}

func TestProfileCalculator_ComputeZScore_ZeroStddev(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	profile := &baseline.BaselineProfile{
		SampleCount: 5,
		Mean:        50.0,
		StdDev:      0.0, // No variance
		Min:         50.0,
		Max:         50.0,
	}

	// When stddev is 0, should return 0 (not NaN)
	zscore := calc.ComputeZScore(50.0, profile)
	assert.Equal(t, 0.0, zscore)

	// Even with different value, should return 0
	zscore = calc.ComputeZScore(100.0, profile)
	assert.Equal(t, 0.0, zscore)
}

func TestProfileCalculator_ComputeZScore_NegativeStddev(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	profile := &baseline.BaselineProfile{
		SampleCount: 5,
		Mean:        50.0,
		StdDev:      -1.0, // Invalid but should handle gracefully
		Min:         50.0,
		Max:         50.0,
	}

	zscore := calc.ComputeZScore(50.0, profile)
	assert.Equal(t, 0.0, zscore) // Should handle gracefully, return 0
}

func TestProfileCalculator_ComputeZScore_NilProfile(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	zscore := calc.ComputeZScore(100.0, nil)
	assert.Equal(t, 0.0, zscore)
}

func TestComputeZScore_Standalone(t *testing.T) {
	profile := &baseline.BaselineProfile{
		SampleCount: 10,
		Mean:        50.0,
		StdDev:      5.0,
		Min:         40.0,
		Max:         60.0,
	}

	zscore := baseline.ComputeZScore(55.0, profile)
	assert.InDelta(t, 1.0, zscore, 0.001)

	zscore = baseline.ComputeZScore(60.0, profile)
	assert.InDelta(t, 2.0, zscore, 0.001)
}

func TestProfileCalculator_ComputeZScore_LargeDeviation(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	profile := &baseline.BaselineProfile{
		SampleCount: 100,
		Mean:        1000.0,
		StdDev:      50.0,
		Min:         850.0,
		Max:         1150.0,
	}

	// Very high value
	zscore := calc.ComputeZScore(1500.0, profile)
	assert.InDelta(t, 10.0, zscore, 0.001) // 10 std devs above mean

	// Very low value
	zscore = calc.ComputeZScore(500.0, profile)
	assert.InDelta(t, -10.0, zscore, 0.001) // 10 std devs below mean
}

func TestProfileCalculator_ComputeZScore_DoesntReturnNaN(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	profile := &baseline.BaselineProfile{
		SampleCount: 1,
		Mean:        0.0,
		StdDev:      0.0,
		Min:         0.0,
		Max:         0.0,
	}

	zscore := calc.ComputeZScore(math.MaxFloat64, profile)
	assert.False(t, math.IsNaN(zscore), "z-score should never be NaN")
	assert.False(t, math.IsInf(zscore, 0), "z-score should never be Inf")
}

func TestProfileCalculator_ComputeProfile_NegativeValues(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{-10.0, -5.0, 0.0, 5.0, 10.0}

	profile, err := calc.ComputeProfile(samples)

	require.NoError(t, err)
	assert.Equal(t, 0.0, profile.Mean)
	assert.Greater(t, profile.StdDev, 0.0)
	assert.Equal(t, -10.0, profile.Min)
	assert.Equal(t, 10.0, profile.Max)
}

func TestProfileCalculator_ComputeProfile_VerySmallValues(t *testing.T) {
	calc := baseline.NewProfileCalculator()

	samples := []float64{0.001, 0.002, 0.003, 0.004, 0.005}

	profile, err := calc.ComputeProfile(samples)

	require.NoError(t, err)
	assert.Greater(t, profile.Mean, 0.0)
	assert.Greater(t, profile.StdDev, 0.0)
	assert.InDelta(t, 0.001, profile.Min, 0.0001)
	assert.InDelta(t, 0.005, profile.Max, 0.0001)
}
