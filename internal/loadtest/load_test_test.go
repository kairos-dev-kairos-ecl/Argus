package loadtest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSignalGeneratorGenerate(t *testing.T) {
	gen := NewSignalGenerator()

	for i := 0; i < 10; i++ {
		signal := gen.Generate()
		assert.NotEmpty(t, signal)
		assert.Contains(t, signal, "signal_id")
		assert.Contains(t, signal, "trace_id")
		assert.Contains(t, signal, "layer")
	}
}

func TestLoadTestBasic(t *testing.T) {
	config := &LoadTestConfig{
		TargetRPS:    100,
		Duration:     500 * time.Millisecond,
		Concurrency:  2,
		HTTPEndpoint: "http://localhost:8080/api/v1/signals",
		HistogramBins: 50,
	}

	lt := NewLoadTest(config, zap.NewNop())
	gen := NewSignalGenerator()

	ctx := context.Background()
	err := lt.Run(ctx, func() string {
		return gen.Generate()
	})

	assert.NoError(t, err)

	// Check that some signals were processed
	results := lt.GetResults(config.Duration)
	assert.Greater(t, results.TotalSignals, int64(0))
	assert.GreaterOrEqual(t, results.AvgLatency, time.Duration(0))
}

func TestLoadTestMetrics(t *testing.T) {
	config := &LoadTestConfig{
		TargetRPS:    50,
		Duration:     300 * time.Millisecond,
		Concurrency:  1,
		HTTPEndpoint: "http://localhost:8080/api/v1/signals",
	}

	lt := NewLoadTest(config, zap.NewNop())
	gen := NewSignalGenerator()

	ctx := context.Background()
	_ = lt.Run(ctx, func() string {
		return gen.Generate()
	})

	results := lt.GetResults(config.Duration)

	assert.Greater(t, results.TotalSignals, int64(0))
	assert.Equal(t, results.SuccessCount+results.ErrorCount, results.TotalSignals)
	assert.GreaterOrEqual(t, results.SignalsPerSec, 0.0)
}

func TestLoadTestTargetRate(t *testing.T) {
	config := &LoadTestConfig{
		TargetRPS:    1000,
		Duration:     1 * time.Second,
		Concurrency:  4,
		HTTPEndpoint: "http://localhost:8080/api/v1/signals",
	}

	lt := NewLoadTest(config, zap.NewNop())
	gen := NewSignalGenerator()

	ctx := context.Background()
	start := time.Now()
	_ = lt.Run(ctx, func() string {
		return gen.Generate()
	})
	actual := time.Since(start)

	results := lt.GetResults(config.Duration)

	// Should have ingested some signals (might not reach exactly 1000 due to overhead)
	assert.Greater(t, results.TotalSignals, int64(100))
	// Actual duration should be close to target (within 500ms buffer for startup/shutdown)
	assert.Less(t, actual, config.Duration+500*time.Millisecond)
}

func TestLoadTestZeroLatency(t *testing.T) {
	config := &LoadTestConfig{
		TargetRPS:    10,
		Duration:     100 * time.Millisecond,
		Concurrency:  1,
		HTTPEndpoint: "http://localhost:8080/api/v1/signals",
	}

	lt := NewLoadTest(config, zap.NewNop())
	gen := NewSignalGenerator()

	ctx := context.Background()
	_ = lt.Run(ctx, func() string {
		return gen.Generate()
	})

	results := lt.GetResults(config.Duration)

	if results.TotalSignals > 0 {
		assert.GreaterOrEqual(t, results.AvgLatency, time.Duration(0))
		assert.GreaterOrEqual(t, results.P95Latency, time.Duration(0))
		assert.GreaterOrEqual(t, results.P99Latency, time.Duration(0))
	}
}

func TestLoadTestConcurrency(t *testing.T) {
	configs := []int{1, 2, 4, 8}

	for _, concurrency := range configs {
		t.Run(fmt.Sprintf("concurrency_%d", concurrency), func(t *testing.T) {
			config := &LoadTestConfig{
				TargetRPS:    100,
				Duration:     200 * time.Millisecond,
				Concurrency:  concurrency,
				HTTPEndpoint: "http://localhost:8080/api/v1/signals",
			}

			lt := NewLoadTest(config, zap.NewNop())
			gen := NewSignalGenerator()

			ctx := context.Background()
			_ = lt.Run(ctx, func() string {
				return gen.Generate()
			})

			results := lt.GetResults(config.Duration)
			assert.Greater(t, results.TotalSignals, int64(0))
		})
	}
}
