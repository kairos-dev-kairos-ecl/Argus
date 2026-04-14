package kairos

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"go.uber.org/zap"
)

// SignalBuilder builds L_DECISION signals from policy evaluation results.
type SignalBuilder struct {
	logger *zap.Logger
}

// NewSignalBuilder creates a new signal builder.
func NewSignalBuilder(logger *zap.Logger) *SignalBuilder {
	return &SignalBuilder{
		logger: logger,
	}
}

// BuildDecisionSignal creates an L_DECISION signal from an evaluation result.
func (sb *SignalBuilder) BuildDecisionSignal(
	traceID string,
	spanID string,
	signalID string,
	ruleID string,
	ruleName string,
	layer string,
	category string,
	result *EvaluationResult,
	appID string,
) *v1.ArgusSignal {
	now := time.Now()
	decisionSignalID := uuid.New().String()

	// Map decision string to proto enum
	var decisionEnum v1.ContextLDecision_Decision
	switch result.Decision {
	case DecisionAllow:
		decisionEnum = v1.ContextLDecision_ALLOW
	case DecisionDeny:
		decisionEnum = v1.ContextLDecision_DENY
	case DecisionReview:
		decisionEnum = v1.ContextLDecision_REVIEW
	default:
		decisionEnum = v1.ContextLDecision_DECISION_UNSPECIFIED
	}

	// Map action string to proto enum
	var actionEnum v1.ContextLDecision_RecommendedAction
	switch result.RecommendedAction {
	case ActionSuppress:
		actionEnum = v1.ContextLDecision_SUPPRESS
	case ActionEscalate:
		actionEnum = v1.ContextLDecision_ESCALATE
	case ActionInvestigate:
		actionEnum = v1.ContextLDecision_INVESTIGATE
	default:
		actionEnum = v1.ContextLDecision_ACTION_UNSPECIFIED
	}

	signal := &v1.ArgusSignal{
		SignalId: decisionSignalID,
		TraceId:  traceID,
		SpanId:   spanID,
		Timestamp: timestamppb.New(now),
		Source: &v1.Source{
			AppId: appID,
		},
		Layer:    v1.Layer_L_DECISION,
		Category: "decision.kairos",
		Context: &v1.ArgusSignal_ContextLDecision{
			ContextLDecision: &v1.ContextLDecision{
				Decision:           decisionEnum,
				Confidence:         float32(result.Confidence),
				Reasoning:          result.Reasoning,
				RecommendedAction:  actionEnum,
				PolicyVersion:      result.KairosPolicyVersion,
				PolicyName:         result.PolicyRuleTriggered,
				EvaluationTimeMs:   float32(result.ProcessingTimeMs),
			},
		},
	}

	sb.logger.Debug("L_DECISION signal built",
		zap.String("signal_id", decisionSignalID),
		zap.String("trace_id", traceID),
		zap.String("decision", result.Decision),
		zap.Float64("confidence", result.Confidence),
	)

	return signal
}

// BuildDecisionSignalFromDetection creates an L_DECISION signal from detection context.
func (sb *SignalBuilder) BuildDecisionSignalFromDetection(
	detectionSignal *v1.ArgusSignal,
	result *EvaluationResult,
	appID string,
) *v1.ArgusSignal {
	// Extract rule info from detection signal (may be empty if not available)
	ruleID := ""
	ruleName := ""
	// Rule metadata would be stored in detection signal's context if available
	// For now, use defaults

	return sb.BuildDecisionSignal(
		detectionSignal.TraceId,
		detectionSignal.SpanId,
		detectionSignal.SignalId,
		ruleID,
		ruleName,
		detectionSignal.Layer.String(),
		detectionSignal.Category,
		result,
		appID,
	)
}

// Decision scores for decision signals.
const (
	DecisionAllow   = "allow"
	DecisionDeny    = "deny"
	DecisionReview  = "review"
)

// RecommendedAction types.
const (
	ActionSuppress   = "suppress"
	ActionEscalate   = "escalate"
	ActionInvestigate = "investigate"
)

// IsDecisionSignal checks if a signal is an L_DECISION signal.
func IsDecisionSignal(signal *v1.ArgusSignal) bool {
	if signal == nil {
		return false
	}
	return signal.Layer == v1.Layer_L_DECISION
}

// ExtractDecision extracts the decision from a signal.
func ExtractDecision(signal *v1.ArgusSignal) string {
	if signal == nil {
		return ""
	}
	ctx := signal.GetContextLDecision()
	if ctx == nil {
		return ""
	}
	return ctx.Decision.String()
}

// ExtractConfidence extracts the confidence score from a signal.
func ExtractConfidence(signal *v1.ArgusSignal) float64 {
	if signal == nil {
		return 0
	}
	ctx := signal.GetContextLDecision()
	if ctx == nil {
		return 0
	}
	return float64(ctx.Confidence)
}

// ExtractReasoning extracts the reasoning from a signal.
func ExtractReasoning(signal *v1.ArgusSignal) string {
	if signal == nil {
		return ""
	}
	ctx := signal.GetContextLDecision()
	if ctx == nil {
		return ""
	}
	return ctx.Reasoning
}

// ExtractRecommendedAction extracts the recommended action from a signal.
func ExtractRecommendedAction(signal *v1.ArgusSignal) string {
	if signal == nil {
		return ""
	}
	ctx := signal.GetContextLDecision()
	if ctx == nil {
		return ""
	}
	return ctx.RecommendedAction.String()
}

// DecisionSignalInfo contains extracted decision signal information.
type DecisionSignalInfo struct {
	Decision          string
	Confidence        float64
	Reasoning         string
	RecommendedAction string
	PolicyVersion     string
	PolicyRule        string
	ProcessingTimeMs  int64
	SourceRuleID      string
	SourceRuleName    string
	SourceSignalID    string
	SourceLayer       string
	SourceCategory    string
}

// ExtractDecisionInfo extracts all decision information from a signal.
func ExtractDecisionInfo(signal *v1.ArgusSignal) *DecisionSignalInfo {
	if !IsDecisionSignal(signal) {
		return nil
	}

	ctx := signal.GetContextLDecision()
	if ctx == nil {
		return nil
	}

	info := &DecisionSignalInfo{
		Decision:          ExtractDecision(signal),
		Confidence:        ExtractConfidence(signal),
		Reasoning:         ExtractReasoning(signal),
		RecommendedAction: ExtractRecommendedAction(signal),
		PolicyVersion:     ctx.PolicyVersion,
		PolicyRule:        ctx.PolicyName,
		ProcessingTimeMs:  int64(ctx.EvaluationTimeMs),
	}

	return info
}
