package kairos

import (
	"testing"
	"time"

	pb "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSignalBuilderBuildDecisionSignal(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	result := &EvaluationResult{
		Decision:            "allow",
		Confidence:          0.95,
		Reasoning:           "Signal matches whitelist",
		RecommendedAction:   "suppress",
		KairosPolicyVersion: "1.0",
		PolicyRuleTriggered: "whitelist-rule",
		ProcessingTimeMs:    10,
	}

	signal := builder.BuildDecisionSignal(
		"trace-123",
		"span-456",
		"signal-789",
		"rule-001",
		"Test Rule",
		"L7",
		"api-call",
		result,
		"app-001",
	)

	assert.NotNil(t, signal)
	assert.NotEmpty(t, signal.SignalId)
	assert.Equal(t, "trace-123", signal.TraceId)
	assert.Equal(t, "span-456", signal.SpanId)
	assert.Equal(t, "app-001", signal.AppId)
	assert.Equal(t, pb.Layer_LAYER_DECISION, signal.Layer)
	assert.NotNil(t, signal.Metadata)
	assert.Equal(t, "allow", signal.Metadata["decision"])
	assert.Equal(t, "0.95", signal.Metadata["confidence"])
	assert.Equal(t, "Signal matches whitelist", signal.Metadata["reasoning"])
	assert.Equal(t, "suppress", signal.Metadata["recommended_action"])
}

func TestIsDecisionSignal(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	result := &EvaluationResult{
		Decision:   "deny",
		Confidence: 0.8,
		Reasoning:  "Suspicious pattern",
	}

	signal := builder.BuildDecisionSignal(
		"trace-123",
		"span-456",
		"signal-789",
		"rule-001",
		"Test Rule",
		"L7",
		"api-call",
		result,
		"app-001",
	)

	assert.True(t, IsDecisionSignal(signal))

	// Create non-decision signal
	otherSignal := &pb.ArgusSignal{
		Layer: pb.Layer_LAYER_APPLICATION,
	}
	assert.False(t, IsDecisionSignal(otherSignal))
	assert.False(t, IsDecisionSignal(nil))
}

func TestExtractDecisionInfo(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	result := &EvaluationResult{
		Decision:            "review",
		Confidence:          0.65,
		Reasoning:           "Requires human review",
		RecommendedAction:   "escalate",
		KairosPolicyVersion: "2.0",
		PolicyRuleTriggered: "review-rule",
		ProcessingTimeMs:    15,
	}

	signal := builder.BuildDecisionSignal(
		"trace-123",
		"span-456",
		"signal-789",
		"rule-001",
		"Test Rule",
		"L7",
		"api-call",
		result,
		"app-001",
	)

	info := ExtractDecisionInfo(signal)
	require.NotNil(t, info)
	assert.Equal(t, "review", info.Decision)
	assert.Equal(t, 0.65, info.Confidence)
	assert.Equal(t, "Requires human review", info.Reasoning)
	assert.Equal(t, "escalate", info.RecommendedAction)
	assert.Equal(t, "2.0", info.PolicyVersion)
	assert.Equal(t, "review-rule", info.PolicyRule)
	assert.Equal(t, int64(15), info.ProcessingTimeMs)
	assert.Equal(t, "rule-001", info.SourceRuleID)
	assert.Equal(t, "Test Rule", info.SourceRuleName)
}

func TestExtractDecisionFunctions(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	result := &EvaluationResult{
		Decision:   "deny",
		Confidence: 0.92,
		Reasoning:  "Malicious behavior detected",
	}

	signal := builder.BuildDecisionSignal(
		"trace-123",
		"span-456",
		"signal-789",
		"rule-001",
		"Test Rule",
		"L7",
		"api-call",
		result,
		"app-001",
	)

	assert.Equal(t, "deny", ExtractDecision(signal))
	assert.Equal(t, 0.92, ExtractConfidence(signal))
	assert.Equal(t, "Malicious behavior detected", ExtractReasoning(signal))
}

func TestBuildDecisionSignalFromDetection(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	// Create a detection signal
	detectionSignal := &pb.ArgusSignal{
		SignalId: "detection-signal-1",
		TraceId:  "trace-123",
		SpanId:   "span-456",
		Layer:    pb.Layer_LAYER_APPLICATION,
		Metadata: map[string]string{
			"rule_id":   "rule-001",
			"rule_name": "Detection Rule",
		},
	}

	result := &EvaluationResult{
		Decision:   "allow",
		Confidence: 0.88,
		Reasoning:  "Approved by policy",
	}

	decisionSignal := builder.BuildDecisionSignalFromDetection(
		detectionSignal,
		result,
		"app-001",
	)

	assert.NotNil(t, decisionSignal)
	assert.Equal(t, detectionSignal.TraceId, decisionSignal.TraceId)
	assert.Equal(t, detectionSignal.SpanId, decisionSignal.SpanId)
	assert.Equal(t, pb.Layer_LAYER_DECISION, decisionSignal.Layer)
	assert.Equal(t, "allow", decisionSignal.Metadata["decision"])
	assert.Equal(t, "rule-001", decisionSignal.Metadata["source_rule_id"])
	assert.Equal(t, "Detection Rule", decisionSignal.Metadata["source_rule_name"])
}

func TestSignalWithTimestamp(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	builder := NewSignalBuilder(logger)

	beforeTime := time.Now()

	result := &EvaluationResult{
		Decision:   "allow",
		Confidence: 0.9,
		Reasoning:  "OK",
	}

	signal := builder.BuildDecisionSignal(
		"trace-123",
		"span-456",
		"signal-789",
		"rule-001",
		"Test Rule",
		"L7",
		"api-call",
		result,
		"app-001",
	)

	afterTime := time.Now()

	assert.NotZero(t, signal.Timestamp)
	assert.True(t, signal.Timestamp >= beforeTime.Unix())
	assert.True(t, signal.Timestamp <= afterTime.Unix()+1) // +1 for any timing issues
	assert.NotZero(t, signal.TimestampNs)
}

func TestNilSignalHandling(t *testing.T) {
	assert.False(t, IsDecisionSignal(nil))
	assert.Equal(t, "", ExtractDecision(nil))
	assert.Equal(t, 0.0, ExtractConfidence(nil))
	assert.Equal(t, "", ExtractReasoning(nil))
	assert.Nil(t, ExtractDecisionInfo(nil))
}
