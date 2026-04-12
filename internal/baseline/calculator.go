package baseline

import (
	"fmt"
	"math"
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BaselineProfile represents statistical baseline metrics for (app_id, layer, category)
// This is a local definition until the proto message is added to signal.proto
type BaselineProfile struct {
	AppID       string
	Layer       v1.Layer
	Category    string
	Mean        float64
	StdDev      float64
	Min         float64
	Max         float64
	SampleCount int32
	ComputedAt  *timestamppb.Timestamp
	ExpiresAt   *timestamppb.Timestamp
}

// ProfileCalculator computes baseline statistics from signal samples.
// It provides mean, standard deviation, min, and max for a dataset,
// and can compute z-scores from these baselines.
type ProfileCalculator struct{}

// NewProfileCalculator creates a new profile calculator.
func NewProfileCalculator() *ProfileCalculator {
	return &ProfileCalculator{}
}

// ComputeProfile analyzes signal samples and returns a baseline profile.
// It computes mean, standard deviation, min, and max.
// Returns error only if input is nil or empty.
func (pc *ProfileCalculator) ComputeProfile(samples []float64) (*BaselineProfile, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("cannot compute profile from empty sample set")
	}

	// Compute min and max
	min := samples[0]
	max := samples[0]
	sum := samples[0]

	for i := 1; i < len(samples); i++ {
		v := samples[i]
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	// Compute mean
	mean := sum / float64(len(samples))

	// Compute variance
	var variance float64
	for _, v := range samples {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(samples))

	// Compute standard deviation
	stddev := math.Sqrt(variance)

	// Build profile
	now := timestamppb.Now()
	expiresAt := timestamppb.New(time.Now().Add(24 * time.Hour))
	profile := &BaselineProfile{
		Mean:        mean,
		StdDev:      stddev,
		Min:         min,
		Max:         max,
		SampleCount: int32(len(samples)),
		ComputedAt:  now,
		ExpiresAt:   expiresAt,
	}

	return profile, nil
}

// ComputeZScore calculates the z-score deviation for a value given a baseline profile.
// Formula: (value - mean) / stddev
// If stddev is 0 or less, returns 0 (no deviation, addresses Pitfall 4).
// This prevents NaN results when all baseline samples are identical.
func (pc *ProfileCalculator) ComputeZScore(value float64, profile *BaselineProfile) float64 {
	if profile == nil {
		return 0
	}

	// If stddev is 0 or negative, return 0 (no meaningful deviation)
	// This prevents division by zero and NaN results
	if profile.StdDev <= 0 {
		return 0
	}

	zscore := (value - profile.Mean) / profile.StdDev

	// Sanity check: ensure result is finite
	if math.IsNaN(zscore) || math.IsInf(zscore, 0) {
		return 0
	}

	return zscore
}

// StandaloneComputeZScore is a package-level function for z-score computation.
// It's a convenience function that doesn't require a ProfileCalculator instance.
func ComputeZScore(value float64, profile *BaselineProfile) float64 {
	if profile == nil {
		return 0
	}

	if profile.StdDev <= 0 {
		return 0
	}

	zscore := (value - profile.Mean) / profile.StdDev
	if math.IsNaN(zscore) || math.IsInf(zscore, 0) {
		return 0
	}

	return zscore
}
