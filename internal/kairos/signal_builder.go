package kairos

import (
	"time"

	argusv1 "github.com/argusxdr/argus/gen/go/argus/v1"
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
) *argusv1.ArgusSignal {
	now := time.Now()

	// Generate signal ID
	signalID := ulid.Make().String()

	// Parse decision string to enum
	decisionEnum := argusv1.ContextLDecision_DECISION_UNSPECIFIED
	switch decision.Decision {
	case "allow":
		decisionEnum = argusv1.ContextLDecision_ALLOW
	case "deny":
		decisionEnum = argusv1.ContextLDecision_DENY
	case "review":
		decisionEnum = argusv1.ContextLDecision_REVIEW
	}

	// Parse recommended action string to enum
	actionEnum := argusv1.ContextLDecision_ACTION_UNSPECIFIED
	switch decision.RecommendedAction {
	case "suppress":
		actionEnum = argusv1.ContextLDecision_SUPPRESS
	case "escalate":
		actionEnum = argusv1.ContextLDecision_ESCALATE
	case "investigate":
		actionEnum = argusv1.ContextLDecision_INVESTIGATE
	}

	// Create the L_DECISION context
	decisionContext := &argusv1.ContextLDecision{
		Decision:          decisionEnum,
		Confidence:        float32(decision.Confidence),
		Reasoning:         decision.Reasoning,
		RecommendedAction: actionEnum,
		PolicyVersion:     decision.PolicyVersion,
		PolicyName:        decision.PolicyName,
		EvaluationTimeMs:  float32(decision.EvaluationTimeMs),
	}

	// Determine severity based on decision
	severity := argusv1.Severity_LOW
	switch decision.Decision {
	case "deny":
		severity = argusv1.Severity_HIGH
	case "review":
		severity = argusv1.Severity_MEDIUM
	case "allow":
		severity = argusv1.Severity_INFO
	}

	// Build the signal
	signal := &argusv1.ArgusSignal{
		SignalId:   signalID,
		TraceId:    traceID,
		SpanId:     uuid.New().String(),
		ParentSpanId: &parentSignalID,
		Source: &argusv1.Source{
			AppId:       sourceAppID,
			Environment: "prod",
		},
		Layer:     argusv1.Layer_L_DECISION,
		Category:  "policy.decision",
		Severity:  severity,
		Timestamp: timestamppb.New(now),
		IngestedAt: timestamppb.New(now),
		Context: &argusv1.ArgusSignal_ContextLDecision{
			ContextLDecision: decisionContext,
		},
		DataClassification: argusv1.DataClassification_INTERNAL,
	}

	return signal
}

// BuildSignalFromRequest creates an L_DECISION signal directly from an evaluation request
// This is useful when Kairos responds with a decision for a specific signal
func (sb *SignalBuilder) BuildSignalFromRequest(
	decision *PolicyDecision,
	req *EvaluationRequest,
	sourceAppID string,
) *argusv1.ArgusSignal {
	return sb.BuildSignal(decision, req.TraceID, req.SignalID, sourceAppID)
}
