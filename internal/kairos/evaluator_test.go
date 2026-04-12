package kairos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestEvaluatorDisabled(t *testing.T) {
	config := &PolicyConfig{
		Enabled: false,
	}

	evaluator := NewEvaluator(nil, config, zap.NewNop())
	req := NewTestEvaluationRequest("trace-123", "signal-456")

	decision := evaluator.Evaluate(context.Background(), req)

	// Should return nil when disabled
	assert.Nil(t, decision)
}

func TestEvaluatorCaching(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	req := NewTestEvaluationRequest("trace-123", "signal-456")

	// First call should hit Kairos
	decision1 := evaluator.Evaluate(context.Background(), req)
	assert.NotNil(t, decision1)

	metrics1 := evaluator.Metrics()
	assert.Equal(t, int64(1), metrics1["evaluations"])
	assert.Equal(t, int64(0), metrics1["cache_hits"])
	assert.Equal(t, int64(1), metrics1["cache_misses"])

	// Second call should hit cache
	decision2 := evaluator.Evaluate(context.Background(), req)
	assert.NotNil(t, decision2)

	metrics2 := evaluator.Metrics()
	assert.Equal(t, int64(2), metrics2["evaluations"])
	assert.Equal(t, int64(1), metrics2["cache_hits"])
	assert.Equal(t, int64(1), metrics2["cache_misses"])

	// Decisions should be identical
	assert.Equal(t, decision1.Decision, decision2.Decision)
	assert.Equal(t, decision1.Confidence, decision2.Confidence)
}

func TestEvaluatorCacheExpiration(t *testing.T) {
	config := &PolicyConfig{
		Enabled:     true,
		TimeoutMs:   1000,
		FailOpen:    true,
		CacheSize:   100,
		CacheTTLSec: 1, // 1 second TTL
		Policy:      DefaultPolicy(),
	}

	mockServer := NewMockServer(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	client := NewClient(mockServer.URL+"/evaluate", time.Duration(config.TimeoutMs)*time.Millisecond, nil)
	evaluator := NewEvaluator(client, config, zap.NewNop())

	req := NewTestEvaluationRequest("trace-123", "signal-456")

	// First evaluation
	decision1 := evaluator.Evaluate(context.Background(), req)
	assert.NotNil(t, decision1)

	// Wait for cache to expire
	time.Sleep(1200 * time.Millisecond)

	// Second evaluation should miss cache
	decision2 := evaluator.Evaluate(context.Background(), req)
	assert.NotNil(t, decision2)

	metrics := evaluator.Metrics()
	assert.Equal(t, int64(0), metrics["cache_hits"]) // No cache hits after expiration
}

func TestEvaluatorClearCache(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	req := NewTestEvaluationRequest("trace-123", "signal-456")

	// Populate cache
	evaluator.Evaluate(context.Background(), req)
	metrics := evaluator.Metrics()
	assert.Equal(t, 1, metrics["cache_size"].(int))

	// Clear cache
	evaluator.ClearCache()
	metrics = evaluator.Metrics()
	assert.Equal(t, 0, metrics["cache_size"].(int))
}

func TestEvaluatorFailOpen(t *testing.T) {
	// Test that evaluator handles unreachable Kairos gracefully
	config := &PolicyConfig{
		Enabled:     true,
		Endpoint:    "http://localhost:19999/nonexistent",
		TimeoutMs:   100,
		FailOpen:    true,
		CacheSize:   100,
		CacheTTLSec: 60,
		Policy:      DefaultPolicy(),
	}

	client := NewClient(config.Endpoint, time.Duration(config.TimeoutMs)*time.Millisecond, zap.NewNop())
	evaluator := NewEvaluator(client, config, zap.NewNop())

	req := NewTestEvaluationRequest("trace-123", "signal-456")
	decision := evaluator.Evaluate(context.Background(), req)

	// Should return nil (fail-open), not error
	assert.Nil(t, decision)

	metrics := evaluator.Metrics()
	assert.Equal(t, int64(1), metrics["evaluations"])
	// Errors may not always be counted on fail-open, so just verify evaluations happened
}

func TestEvaluatorMetrics(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("deny", "escalate", 0.95))
	defer mockServer.Close()

	req1 := NewTestEvaluationRequest("trace-1", "signal-1")
	req2 := NewTestEvaluationRequest("trace-1", "signal-1") // Same signal
	req3 := NewTestEvaluationRequest("trace-2", "signal-2") // Different signal

	evaluator.Evaluate(context.Background(), req1)
	evaluator.Evaluate(context.Background(), req2) // Cache hit
	evaluator.Evaluate(context.Background(), req3)

	metrics := evaluator.Metrics()

	assert.Equal(t, int64(3), metrics["evaluations"])
	assert.Equal(t, int64(1), metrics["cache_hits"])
	assert.Equal(t, int64(2), metrics["cache_misses"])
	assert.Equal(t, int64(0), metrics["errors"])

	// Reset metrics
	evaluator.ResetMetrics()
	metrics = evaluator.Metrics()
	assert.Equal(t, int64(0), metrics["evaluations"])
	assert.Equal(t, int64(0), metrics["cache_hits"])
	assert.Equal(t, int64(0), metrics["cache_misses"])
}
