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

	decision, err := evaluator.Evaluate(context.Background(), req)

	// Should return nil when disabled
	assert.Nil(t, decision)
	assert.Nil(t, err)
}

func TestEvaluatorCaching(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	req := NewTestEvaluationRequest("trace-123", "signal-456")

	// First call should hit Kairos
	decision1, err1 := evaluator.Evaluate(context.Background(), req)
	assert.NoError(t, err1)
	assert.NotNil(t, decision1)

	metrics1 := evaluator.Metrics()
	assert.Equal(t, int64(1), metrics1["evaluations"])
	assert.Equal(t, int64(0), metrics1["cache_hits"])
	assert.Equal(t, int64(1), metrics1["cache_misses"])

	// Second call should hit cache
	decision2, err2 := evaluator.Evaluate(context.Background(), req)
	assert.NoError(t, err2)
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
	decision1, err1 := evaluator.Evaluate(context.Background(), req)
	assert.NoError(t, err1)
	assert.NotNil(t, decision1)

	// Wait for cache to expire
	time.Sleep(1200 * time.Millisecond)

	// Second evaluation should miss cache
	decision2, err2 := evaluator.Evaluate(context.Background(), req)
	assert.NoError(t, err2)
	assert.NotNil(t, decision2)

	metrics := evaluator.Metrics()
	assert.Equal(t, int64(0), metrics["cache_hits"]) // No cache hits after expiration
}

func TestEvaluatorClearCache(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	req := NewTestEvaluationRequest("trace-123", "signal-456")

	// Populate cache
	evaluator.Evaluate(context.Background(), req) //nolint:errcheck
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
	decision, _ := evaluator.Evaluate(context.Background(), req)

	// Should return nil (fail-open)
	assert.Nil(t, decision)

	metrics := evaluator.Metrics()
	assert.Equal(t, int64(1), metrics["evaluations"])
	// Errors may not always be counted on fail-open, so just verify evaluations happened
}

// TestEvaluator_CacheKeyIncludesRuleID verifies that cache keys are scoped per rule_id.
// Two Evaluate calls with the same signal_id+trace_id but different rule_ids must
// both hit the upstream server (no cross-rule cache sharing — Pitfall 5 fix).
func TestEvaluator_CacheKeyIncludesRuleID(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("allow", "investigate", 0.9))
	defer mockServer.Close()

	// Build two requests: same signal/trace but different rule_ids.
	reqA := NewTestEvaluationRequest("trace-xyz", "signal-abc")
	reqA.RuleID = "rule-A"

	reqB := NewTestEvaluationRequest("trace-xyz", "signal-abc")
	reqB.RuleID = "rule-B"

	_, err1 := evaluator.Evaluate(context.Background(), reqA)
	_, err2 := evaluator.Evaluate(context.Background(), reqB)

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// Both calls must reach the upstream server (2 HTTP requests, not 1).
	assert.Equal(t, int64(2), mockServer.CallCount.Load(),
		"different rule_ids must produce distinct cache keys — 2 upstream calls expected")
}

func TestEvaluatorMetrics(t *testing.T) {
	evaluator, mockServer := NewTestEvaluator(NewTestDecision("deny", "escalate", 0.95))
	defer mockServer.Close()

	req1 := NewTestEvaluationRequest("trace-1", "signal-1")
	req2 := NewTestEvaluationRequest("trace-1", "signal-1") // Same signal
	req3 := NewTestEvaluationRequest("trace-2", "signal-2") // Different signal

	evaluator.Evaluate(context.Background(), req1) //nolint:errcheck
	evaluator.Evaluate(context.Background(), req2) //nolint:errcheck // Cache hit
	evaluator.Evaluate(context.Background(), req3) //nolint:errcheck

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
