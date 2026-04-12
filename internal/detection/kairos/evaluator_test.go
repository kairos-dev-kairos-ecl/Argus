package kairos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestEvaluatorEvaluateDetection(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	evaluator, mockServer, err := NewTestEvaluator(logger)
	require.NoError(t, err)
	defer mockServer.Close()

	result := evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-1",
		"test-trace-1",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{"endpoint": "/api/users"},
	)

	assert.Equal(t, "allow", result.Decision)
	assert.Equal(t, 0.9, result.Confidence)
	assert.NotEmpty(t, result.Reasoning)
}

func TestEvaluatorDisabled(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	evaluator, mockServer, err := NewTestEvaluator(logger)
	require.NoError(t, err)
	defer mockServer.Close()

	evaluator.SetEnabled(false)

	result := evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-1",
		"test-trace-1",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{},
	)

	// Should allow when disabled
	assert.Equal(t, "allow", result.Decision)
}

func TestEvaluatorFailOpen(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	// Create evaluator with empty endpoint (will fail)
	registry := NewPolicyRegistry(logger)
	policy := NewPolicy("test-policy", "1.0", "Test Policy", true, logger)
	rule := Rule{
		ID:         "test-rule-1",
		Name:       "Test Rule 1",
		Condition:  "layer == 'L7'",
		Action:     "allow",
		Priority:   1,
		Confidence: 0.9,
	}
	policy.AddRule(rule)
	registry.RegisterPolicy(policy)

	client := NewClient("http://localhost:9999", 1, logger) // Invalid endpoint
	evaluator := NewEvaluator(client, registry, "test-policy", true, true, logger)

	result := evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-1",
		"test-trace-1",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{},
	)

	// Should fail-open: allow when Kairos unreachable
	assert.Equal(t, "allow", result.Decision)
	assert.NotEmpty(t, result.Error)
}

func TestEvaluatorFailClosed(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	// Create evaluator with fail-closed behavior
	registry := NewPolicyRegistry(logger)
	policy := NewPolicy("test-policy", "1.0", "Test Policy", true, logger)
	rule := Rule{
		ID:         "test-rule-1",
		Name:       "Test Rule 1",
		Condition:  "layer == 'L7'",
		Action:     "allow",
		Priority:   1,
		Confidence: 0.9,
	}
	policy.AddRule(rule)
	registry.RegisterPolicy(policy)

	client := NewClient("http://localhost:9999", 1, logger) // Invalid endpoint
	evaluator := NewEvaluator(client, registry, "test-policy", true, false, logger) // fail-closed

	result := evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-1",
		"test-trace-1",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{},
	)

	// Should fail-closed: review/deny when Kairos unreachable
	assert.Equal(t, "review", result.Decision)
	assert.NotEmpty(t, result.Error)
}

func TestEvaluatorSetDefaultPolicy(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	evaluator, mockServer, err := NewTestEvaluator(logger)
	require.NoError(t, err)
	defer mockServer.Close()

	// Create second policy
	policy2 := NewPolicy("policy-2", "1.0", "Policy 2", true, logger)
	rule := Rule{
		ID:         "rule-2",
		Name:       "Rule 2",
		Condition:  "category == 'security'",
		Action:     "deny",
		Priority:   1,
		Confidence: 0.85,
	}
	policy2.AddRule(rule)
	evaluator.policyRegistry.RegisterPolicy(policy2)

	// Change default policy
	err = evaluator.SetDefaultPolicy("policy-2")
	require.NoError(t, err)
	assert.Equal(t, "policy-2", evaluator.GetDefaultPolicy())

	// Test with non-existent policy
	err = evaluator.SetDefaultPolicy("non-existent")
	assert.Error(t, err)
}

func TestEvaluatorHealth(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	evaluator, mockServer, err := NewTestEvaluator(logger)
	require.NoError(t, err)
	defer mockServer.Close()

	err = evaluator.Health(context.Background())
	assert.NoError(t, err)

	// Disable and check
	evaluator.SetEnabled(false)
	err = evaluator.Health(context.Background())
	assert.Error(t, err)
}
