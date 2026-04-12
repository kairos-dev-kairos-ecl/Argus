package kairos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClientEvaluatePolicy(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	mockServer, err := NewMockKairosServer()
	require.NoError(t, err)
	defer mockServer.Close()

	client := NewClient(mockServer.GetEndpoint(), 5*time.Second, logger)

	req := &PolicyRequest{
		SignalID:   "test-signal-1",
		TraceID:    "test-trace-1",
		Layer:      "L7",
		Category:   "api-call",
		RuleID:     "test-rule-1",
		RuleName:   "Test Rule",
		Confidence: 0.8,
		Data: map[string]interface{}{
			"endpoint": "/api/users",
			"method":   "POST",
		},
		Timestamp: time.Now().Unix(),
	}

	resp, err := client.EvaluatePolicy(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "allow", resp.Decision)
	assert.Equal(t, 1, mockServer.CallCount)
}

func TestClientTimeout(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	mockServer, err := NewMockKairosServer()
	require.NoError(t, err)
	defer mockServer.Close()

	client := NewClient(mockServer.GetEndpoint(), 1*time.Millisecond, logger)

	req := &PolicyRequest{
		SignalID:   "test-signal-1",
		TraceID:    "test-trace-1",
		Layer:      "L7",
		Category:   "api-call",
		RuleID:     "test-rule-1",
		RuleName:   "Test Rule",
		Confidence: 0.8,
		Data:       make(map[string]interface{}),
		Timestamp:  time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = client.EvaluatePolicy(ctx, req)
	// Either context timeout or client timeout should occur
	assert.Error(t, err)
}

func TestClientHealth(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	mockServer, err := NewMockKairosServer()
	require.NoError(t, err)
	defer mockServer.Close()

	client := NewClient(mockServer.GetEndpoint(), 5*time.Second, logger)

	err = client.Health(context.Background())
	assert.NoError(t, err)
}

func TestClientNoEndpoint(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	client := NewClient("", 5*time.Second, logger)

	req := &PolicyRequest{
		SignalID:   "test-signal-1",
		TraceID:    "test-trace-1",
		Layer:      "L7",
		Category:   "api-call",
		RuleID:     "test-rule-1",
		RuleName:   "Test Rule",
		Confidence: 0.8,
		Data:       make(map[string]interface{}),
		Timestamp:  time.Now().Unix(),
	}

	_, err := client.EvaluatePolicy(context.Background(), req)
	assert.Error(t, err)
	assert.Equal(t, "kairos endpoint not configured", err.Error())
}

func TestClientCustomResponse(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	mockServer, err := NewMockKairosServer()
	require.NoError(t, err)
	defer mockServer.Close()

	// Set custom response
	customResp := &PolicyResponse{
		Decision:            "deny",
		Confidence:          0.95,
		Reasoning:           "Suspicious activity detected",
		RecommendedAction:   "escalate",
		KairosPolicyVersion: "2.0",
		PolicyRuleTriggered: "suspicious-pattern-rule",
		ProcessingTimeMs:    15,
	}
	mockServer.SetResponse("test-rule-2", customResp)

	client := NewClient(mockServer.GetEndpoint(), 5*time.Second, logger)

	req := &PolicyRequest{
		SignalID:   "test-signal-2",
		TraceID:    "test-trace-2",
		Layer:      "L7",
		Category:   "api-call",
		RuleID:     "test-rule-2",
		RuleName:   "Suspicious Pattern",
		Confidence: 0.85,
		Data: map[string]interface{}{
			"pattern": "brute-force",
		},
		Timestamp: time.Now().Unix(),
	}

	resp, err := client.EvaluatePolicy(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "deny", resp.Decision)
	assert.Equal(t, 0.95, resp.Confidence)
	assert.Equal(t, "escalate", resp.RecommendedAction)
}
