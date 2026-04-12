package kairos

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	pb "github.com/argusxdr/argus/gen/go/argus/v1"
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
) *pb.ArgusSignal {
	now := time.Now()
	decisionSignalID := uuid.New().String()

	signal := &pb.ArgusSignal{
		SignalId:    decisionSignalID,
		TraceId:     traceID,
		SpanId:      spanID,
		Timestamp:   now.Unix(),
		TimestampNs: int64(now.Nanosecond()),
		AppId:       appID,
		Layer:       pb.Layer_LAYER_DECISION, // L_DECISION layer
		Category:    pb.Category_CATEGORY_SECURITY, // Security category

		// Decision-specific fields
		Metadata: map[string]string{
			"decision":                  result.Decision,
			"confidence":                fmt.Sprintf("%.2f", result.Confidence),
			"reasoning":                 result.Reasoning,
			"recommended_action":        result.RecommendedAction,
			"kairos_policy_version":     result.KairosPolicyVersion,
			"policy_rule_triggered":     result.PolicyRuleTriggered,
			"processing_time_ms":        fmt.Sprintf("%d", result.ProcessingTimeMs),
			"source_rule_id":            ruleID,
			"source_rule_name":          ruleName,
			"source_signal_id":          signalID,
			"source_layer":              layer,
			"source_category":           category,
		},

		// For query and analysis
		Title: fmt.Sprintf("Policy Decision: %s", result.Decision),
		Body:  fmt.Sprintf("Kairos policy evaluation: %s\nReasoning: %s\nRecommended action: %s",
			result.Decision, result.Reasoning, result.RecommendedAction),
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
	detectionSignal *pb.ArgusSignal,
	result *EvaluationResult,
	appID string,
) *pb.ArgusSignal {
	return sb.BuildDecisionSignal(
		detectionSignal.TraceId,
		detectionSignal.SpanId,
		detectionSignal.SignalId,
		detectionSignal.Metadata["rule_id"],
		detectionSignal.Metadata["rule_name"],
		detectionSignal.Layer.String(),
		detectionSignal.Category.String(),
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
func IsDecisionSignal(signal *pb.ArgusSignal) bool {
	if signal == nil {
		return false
	}
	return signal.Layer == pb.Layer_LAYER_DECISION
}

// ExtractDecision extracts the decision from a signal.
func ExtractDecision(signal *pb.ArgusSignal) string {
	if signal == nil || signal.Metadata == nil {
		return ""
	}
	return signal.Metadata["decision"]
}

// ExtractConfidence extracts the confidence score from a signal.
func ExtractConfidence(signal *pb.ArgusSignal) float64 {
	if signal == nil || signal.Metadata == nil {
		return 0
	}
	confidenceStr := signal.Metadata["confidence"]
	if confidenceStr == "" {
		return 0
	}
	var conf float64
	fmt.Sscanf(confidenceStr, "%f", &conf)
	return conf
}

// ExtractReasoning extracts the reasoning from a signal.
func ExtractReasoning(signal *pb.ArgusSignal) string {
	if signal == nil || signal.Metadata == nil {
		return ""
	}
	return signal.Metadata["reasoning"]
}

// ExtractRecommendedAction extracts the recommended action from a signal.
func ExtractRecommendedAction(signal *pb.ArgusSignal) string {
	if signal == nil || signal.Metadata == nil {
		return ""
	}
	return signal.Metadata["recommended_action"]
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
func ExtractDecisionInfo(signal *pb.ArgusSignal) *DecisionSignalInfo {
	if !IsDecisionSignal(signal) {
		return nil
	}

	info := &DecisionSignalInfo{
		Decision:          ExtractDecision(signal),
		Confidence:        ExtractConfidence(signal),
		Reasoning:         ExtractReasoning(signal),
		RecommendedAction: ExtractRecommendedAction(signal),
	}

	if signal.Metadata != nil {
		if v, ok := signal.Metadata["kairos_policy_version"]; ok {
			info.PolicyVersion = v
		}
		if v, ok := signal.Metadata["policy_rule_triggered"]; ok {
			info.PolicyRule = v
		}
		if v, ok := signal.Metadata["processing_time_ms"]; ok {
			fmt.Sscanf(v, "%d", &info.ProcessingTimeMs)
		}
		if v, ok := signal.Metadata["source_rule_id"]; ok {
			info.SourceRuleID = v
		}
		if v, ok := signal.Metadata["source_rule_name"]; ok {
			info.SourceRuleName = v
		}
		if v, ok := signal.Metadata["source_signal_id"]; ok {
			info.SourceSignalID = v
		}
		if v, ok := signal.Metadata["source_layer"]; ok {
			info.SourceLayer = v
		}
		if v, ok := signal.Metadata["source_category"]; ok {
			info.SourceCategory = v
		}
	}

	return info
}
