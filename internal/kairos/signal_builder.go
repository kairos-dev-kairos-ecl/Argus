package kairos

import (
	"time"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SignalBuilder creates L_DECISION signals from policy decisions
type SignalBuilder struct{}

// NewSignalBuilder creates a new signal builder
func NewSignalBuilder() *SignalBuilder {
	return &SignalBuilder{}
}

// BuildSignal creates an L_DECISION signal from a policy decision
func (sb *SignalBuilder) BuildSignal(
	decision *PolicyDecision,
	traceID string,
	parentSignalID string,
	sourceAppID string,
) *v1.ArgusSignal {
	now := time.Now()

	// Generate signal ID
	signalID := ulid.Make().String()

	// Parse decision string to enum
	decisionEnum := v1.ContextLDecision_DECISION_UNSPECIFIED
	switch decision.Decision {
	case "allow":
		decisionEnum = v1.ContextLDecision_ALLOW
	case "deny":
		decisionEnum = v1.ContextLDecision_DENY
	case "review":
		decisionEnum = v1.ContextLDecision_REVIEW
	}

	// Parse recommended action string to enum
	actionEnum := v1.ContextLDecision_ACTION_UNSPECIFIED
	switch decision.RecommendedAction {
	case "suppress":
		actionEnum = v1.ContextLDecision_SUPPRESS
	case "escalate":
		actionEnum = v1.ContextLDecision_ESCALATE
	case "investigate":
		actionEnum = v1.ContextLDecision_INVESTIGATE
	}

	// Create the L_DECISION context
	decisionContext := &v1.ContextLDecision{
		Decision:          decisionEnum,
		Confidence:        float32(decision.Confidence),
		Reasoning:         decision.Reasoning,
		RecommendedAction: actionEnum,
		PolicyVersion:     decision.PolicyVersion,
		PolicyName:        decision.PolicyName,
		EvaluationTimeMs:  float32(decision.EvaluationTimeMs),
	}

	// Determine severity based on decision
	severity := v1.Severity_LOW
	switch decision.Decision {
	case "deny":
		severity = v1.Severity_HIGH
	case "review":
		severity = v1.Severity_MEDIUM
	case "allow":
		severity = v1.Severity_INFO
	}

	// Build the signal
	signal := &v1.ArgusSignal{
		SignalId:   signalID,
		TraceId:    traceID,
		SpanId:     uuid.New().String(),
		ParentSpanId: &parentSignalID,
		Source: &v1.Source{
			AppId:       sourceAppID,
			Environment: "prod",
		},
		Layer:     v1.Layer_L_DECISION,
		Category:  "policy.decision",
		Severity:  severity,
		Timestamp: timestamppb.New(now),
		IngestedAt: timestamppb.New(now),
		Context: &v1.ArgusSignal_ContextLDecision{
			ContextLDecision: decisionContext,
		},
		DataClassification: v1.DataClassification_INTERNAL,
	}

	return signal
}

// BuildSignalFromRequest creates an L_DECISION signal directly from an evaluation request
// This is useful when Kairos responds with a decision for a specific signal
func (sb *SignalBuilder) BuildSignalFromRequest(
	decision *PolicyDecision,
	req *EvaluationRequest,
	sourceAppID string,
) *v1.ArgusSignal {
	return sb.BuildSignal(decision, req.TraceID, req.SignalID, sourceAppID)
}
