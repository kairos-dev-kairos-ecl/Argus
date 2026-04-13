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
	builder := NewSignalBuilder(zap.NewNop())

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
		"trace-123", "span-456", "signal-789",
		"rule-001", "Test Rule", "L7", "api-call",
		result, "app-001",
	)

	require.NotNil(t, signal)
	assert.NotEmpty(t, signal.SignalId)
	assert.Equal(t, "trace-123", signal.TraceId)
	assert.Equal(t, "span-456", signal.SpanId)
	assert.Equal(t, "app-001", signal.Source.GetAppId())
	assert.Equal(t, pb.Layer_L_DECISION, signal.Layer)

	ctx := signal.GetContextLDecision()
	require.NotNil(t, ctx)
	assert.Equal(t, pb.ContextLDecision_ALLOW, ctx.Decision)
	assert.InDelta(t, 0.95, float64(ctx.Confidence), 0.001)
	assert.Equal(t, "Signal matches whitelist", ctx.Reasoning)
	assert.Equal(t, pb.ContextLDecision_SUPPRESS, ctx.RecommendedAction)
	assert.Equal(t, "1.0", ctx.PolicyVersion)
	assert.Equal(t, "whitelist-rule", ctx.PolicyName)
	assert.InDelta(t, 10.0, float64(ctx.EvaluationTimeMs), 0.001)
}

func TestIsDecisionSignal(t *testing.T) {
	builder := NewSignalBuilder(zap.NewNop())

	result := &EvaluationResult{Decision: "deny", Confidence: 0.8, Reasoning: "Suspicious pattern"}
	signal := builder.BuildDecisionSignal(
		"trace-123", "span-456", "signal-789",
		"rule-001", "Test Rule", "L7", "api-call",
		result, "app-001",
	)

	assert.True(t, IsDecisionSignal(signal))

	otherSignal := &pb.ArgusSignal{Layer: pb.Layer_L10_APPLICATION}
	assert.False(t, IsDecisionSignal(otherSignal))
	assert.False(t, IsDecisionSignal(nil))
}

func TestExtractDecisionInfo(t *testing.T) {
	builder := NewSignalBuilder(zap.NewNop())

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
		"trace-123", "span-456", "signal-789",
		"rule-001", "Test Rule", "L7", "api-call",
		result, "app-001",
	)

	info := ExtractDecisionInfo(signal)
	require.NotNil(t, info)
	assert.Equal(t, "REVIEW", info.Decision)
	assert.InDelta(t, 0.65, info.Confidence, 0.001)
	assert.Equal(t, "Requires human review", info.Reasoning)
	assert.Equal(t, "ESCALATE", info.RecommendedAction)
	assert.Equal(t, "2.0", info.PolicyVersion)
	assert.Equal(t, "review-rule", info.PolicyRule)
	assert.Equal(t, int64(15), info.ProcessingTimeMs)
}

func TestExtractDecisionFunctions(t *testing.T) {
	builder := NewSignalBuilder(zap.NewNop())

	result := &EvaluationResult{
		Decision:   "deny",
		Confidence: 0.92,
		Reasoning:  "Malicious behavior detected",
	}
	signal := builder.BuildDecisionSignal(
		"trace-123", "span-456", "signal-789",
		"rule-001", "Test Rule", "L7", "api-call",
		result, "app-001",
	)

	assert.Equal(t, "DENY", ExtractDecision(signal))
	assert.InDelta(t, 0.92, ExtractConfidence(signal), 0.001)
	assert.Equal(t, "Malicious behavior detected", ExtractReasoning(signal))
}

func TestBuildDecisionSignalFromDetection(t *testing.T) {
	builder := NewSignalBuilder(zap.NewNop())

	detectionSignal := &pb.ArgusSignal{
		SignalId: "detection-signal-1",
		TraceId:  "trace-123",
		SpanId:   "span-456",
		Layer:    pb.Layer_L10_APPLICATION,
	}

	result := &EvaluationResult{
		Decision:   "allow",
		Confidence: 0.88,
		Reasoning:  "Approved by policy",
	}

	decisionSignal := builder.BuildDecisionSignalFromDetection(detectionSignal, result, "app-001")

	require.NotNil(t, decisionSignal)
	assert.Equal(t, detectionSignal.TraceId, decisionSignal.TraceId)
	assert.Equal(t, detectionSignal.SpanId, decisionSignal.SpanId)
	assert.Equal(t, pb.Layer_L_DECISION, decisionSignal.Layer)
	ctx := decisionSignal.GetContextLDecision()
	require.NotNil(t, ctx)
	assert.Equal(t, pb.ContextLDecision_ALLOW, ctx.Decision)
}

func TestSignalWithTimestamp(t *testing.T) {
	builder := NewSignalBuilder(zap.NewNop())

	beforeTime := time.Now()
	result := &EvaluationResult{Decision: "allow", Confidence: 0.9, Reasoning: "OK"}
	signal := builder.BuildDecisionSignal(
		"trace-123", "span-456", "signal-789",
		"rule-001", "Test Rule", "L7", "api-call",
		result, "app-001",
	)
	afterTime := time.Now()

	require.NotNil(t, signal.Timestamp)
	assert.True(t, signal.Timestamp.IsValid())
	ts := signal.Timestamp.AsTime()
	assert.False(t, ts.Before(beforeTime.Add(-time.Second)))
	assert.False(t, ts.After(afterTime.Add(time.Second)))
}

func TestNilSignalHandling(t *testing.T) {
	assert.False(t, IsDecisionSignal(nil))
	assert.Equal(t, "", ExtractDecision(nil))
	assert.Equal(t, 0.0, ExtractConfidence(nil))
	assert.Equal(t, "", ExtractReasoning(nil))
	assert.Nil(t, ExtractDecisionInfo(nil))
}
