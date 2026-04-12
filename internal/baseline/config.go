package baseline

import (
	"fmt"
	"time"
)

// BaselineConfig holds all tunable parameters for baseline computation.
type BaselineConfig struct {
	// Enabled toggles baseline computation on/off
	Enabled bool

	// ComputeInterval is how often to run baseline computation (default: 10 minutes)
	ComputeInterval time.Duration

	// HistoryWindow is the lookback period for historical samples (default: 7 days)
	HistoryWindow time.Duration

	// MinSamples is the minimum number of samples required to compute a profile (default: 100)
	MinSamples int64

	// AsyncMode indicates whether baseline computation blocks or runs async (default: true)
	AsyncMode bool

	// RedisCacheTTL is the time-to-live for profiles in Redis (default: 5 minutes)
	RedisCacheTTL time.Duration
}

// DefaultBaselineConfig returns conservative, production-tested defaults.
func DefaultBaselineConfig() *BaselineConfig {
	return &BaselineConfig{
		Enabled:         true,
		ComputeInterval: 10 * time.Minute,
		HistoryWindow:   24 * time.Hour, // Default to 24 hours for initial profiles
		MinSamples:      100,
		AsyncMode:       true,
		RedisCacheTTL:   5 * time.Minute,
	}
}

// Validate checks that the configuration has reasonable values.
// Returns error if any required parameter is invalid.
func (bc *BaselineConfig) Validate() error {
	if bc.ComputeInterval < 1*time.Minute {
		return fmt.Errorf("ComputeInterval must be >= 1 minute, got %v", bc.ComputeInterval)
	}

	if bc.HistoryWindow < 1*time.Hour {
		return fmt.Errorf("HistoryWindow must be >= 1 hour, got %v", bc.HistoryWindow)
	}

	if bc.MinSamples < 1 {
		return fmt.Errorf("MinSamples must be >= 1, got %d", bc.MinSamples)
	}

	if bc.RedisCacheTTL < 1*time.Minute {
		return fmt.Errorf("RedisCacheTTL must be >= 1 minute, got %v", bc.RedisCacheTTL)
	}

	return nil
}
