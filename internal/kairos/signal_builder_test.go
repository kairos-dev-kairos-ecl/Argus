package kairos

import (
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
)

func TestSignalBuilderBuildSignal(t *testing.T) {
	builder := NewSignalBuilder()

	decision := &PolicyDecision{
		Decision:          "deny",
		Confidence:        0.95,
		Reasoning:         "Critical severity signal blocked",
		RecommendedAction: "escalate",
		PolicyVersion:     "1.0.0",
		PolicyName:        "default",
		EvaluationTimeMs:  15.5,
	}

	signal := builder.BuildSignal(
		decision,
		"trace-123",
		"signal-456",
		"app-test",
	)

	// Verify signal properties
	assert.NotEmpty(t, signal.SignalId)
	assert.Equal(t, "trace-123", signal.TraceId)
	assert.Equal(t, "signal-456", *signal.ParentSpanId)
	assert.Equal(t, v1.Layer_L_DECISION, signal.Layer)
	assert.Equal(t, "policy.decision", signal.Category)
	assert.Equal(t, v1.Severity_HIGH, signal.Severity) // HIGH for deny

	// Verify decision context
	decisionCtx := signal.GetContextLDecision()
	assert.NotNil(t, decisionCtx)
	assert.Equal(t, v1.ContextLDecision_DENY, decisionCtx.Decision)
	assert.Equal(t, float32(0.95), decisionCtx.Confidence)
	assert.Equal(t, "Critical severity signal blocked", decisionCtx.Reasoning)
	assert.Equal(t, v1.ContextLDecision_ESCALATE, decisionCtx.RecommendedAction)
	assert.Equal(t, "1.0.0", decisionCtx.PolicyVersion)
	assert.Equal(t, "default", decisionCtx.PolicyName)
}

func TestSignalBuilderAllowDecision(t *testing.T) {
	builder := NewSignalBuilder()

	decision := NewTestDecision("allow", "suppress", 1.0)

	signal := builder.BuildSignal(decision, "trace-1", "signal-1", "app-1")

	decisionCtx := signal.GetContextLDecision()
	assert.Equal(t, v1.ContextLDecision_ALLOW, decisionCtx.Decision)
	assert.Equal(t, v1.Severity_INFO, signal.Severity) // INFO for allow
	assert.Equal(t, v1.ContextLDecision_SUPPRESS, decisionCtx.RecommendedAction)
}

func TestSignalBuilderReviewDecision(t *testing.T) {
	builder := NewSignalBuilder()

	decision := NewTestDecision("review", "investigate", 0.8)

	signal := builder.BuildSignal(decision, "trace-2", "signal-2", "app-2")

	decisionCtx := signal.GetContextLDecision()
	assert.Equal(t, v1.ContextLDecision_REVIEW, decisionCtx.Decision)
	assert.Equal(t, v1.Severity_MEDIUM, signal.Severity) // MEDIUM for review
	assert.Equal(t, v1.ContextLDecision_INVESTIGATE, decisionCtx.RecommendedAction)
}

func TestSignalBuilderFromRequest(t *testing.T) {
	builder := NewSignalBuilder()

	decision := NewTestDecision("deny", "escalate", 0.92)
	req := NewTestEvaluationRequest("trace-abc", "signal-xyz")

	signal := builder.BuildSignalFromRequest(decision, req, "app-production")

	assert.Equal(t, "trace-abc", signal.TraceId)
	assert.Equal(t, "signal-xyz", *signal.ParentSpanId)
	assert.Equal(t, "app-production", signal.Source.AppId)
	assert.Equal(t, v1.Layer_L_DECISION, signal.Layer)

	decisionCtx := signal.GetContextLDecision()
	assert.Equal(t, v1.ContextLDecision_DENY, decisionCtx.Decision)
}

func TestSignalBuilderMultipleSignals(t *testing.T) {
	builder := NewSignalBuilder()

	decisions := []*PolicyDecision{
		NewTestDecision("allow", "suppress", 1.0),
		NewTestDecision("review", "investigate", 0.75),
		NewTestDecision("deny", "escalate", 0.95),
	}

	signals := make([]*v1.ArgusSignal, len(decisions))
	for i, decision := range decisions {
		signals[i] = builder.BuildSignal(decision, "trace-1", "signal-"+string(rune(i)), "app-1")
	}

	// Verify all signals have unique IDs
	ids := make(map[string]bool)
	for _, signal := range signals {
		assert.False(t, ids[signal.SignalId], "Duplicate signal ID")
		ids[signal.SignalId] = true
	}

	assert.Equal(t, len(decisions), len(ids))
}

func TestSignalBuilderTimestamps(t *testing.T) {
	builder := NewSignalBuilder()

	decision := NewTestDecision("allow", "suppress", 1.0)
	signal := builder.BuildSignal(decision, "trace-1", "signal-1", "app-1")

	// Verify timestamps are set
	assert.NotNil(t, signal.Timestamp)
	assert.NotNil(t, signal.IngestedAt)
	assert.Greater(t, signal.Timestamp.Seconds, int64(0))
	assert.Greater(t, signal.IngestedAt.Seconds, int64(0))
}
