package kairos

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// EvaluationResult represents the result of policy evaluation.
type EvaluationResult struct {
	Decision            string
	Confidence          float64
	Reasoning           string
	RecommendedAction   string
	KairosPolicyVersion string
	PolicyRuleTriggered string
	ProcessingTimeMs    int64
	Error               string
}

// Evaluator evaluates detection rules against Kairos policies.
type Evaluator struct {
	client          *Client
	policyRegistry  *PolicyRegistry
	defaultPolicy   string
	enabled         bool
	failOpen        bool // If true, continue when Kairos fails
	logger          *zap.Logger
}

// NewEvaluator creates a new policy evaluator.
func NewEvaluator(
	client *Client,
	policyRegistry *PolicyRegistry,
	defaultPolicy string,
	enabled bool,
	failOpen bool,
	logger *zap.Logger,
) *Evaluator {
	return &Evaluator{
		client:         client,
		policyRegistry: policyRegistry,
		defaultPolicy:  defaultPolicy,
		enabled:        enabled,
		failOpen:       failOpen,
		logger:         logger,
	}
}

// EvaluateDetection evaluates a detection rule against configured policies.
func (e *Evaluator) EvaluateDetection(
	ctx context.Context,
	signalID string,
	traceID string,
	layer string,
	category string,
	ruleID string,
	ruleName string,
	confidence float64,
	data map[string]interface{},
) *EvaluationResult {
	result := &EvaluationResult{
		Decision: "allow", // Default: allow
	}

	// If Kairos not enabled, return allow decision
	if !e.enabled {
		e.logger.Debug("kairos evaluation skipped (disabled)",
			zap.String("signal_id", signalID),
			zap.String("rule_id", ruleID),
		)
		return result
	}

	// If no endpoint configured, log and return allow (fail-open)
	if e.client.endpoint == "" {
		e.logger.Warn("kairos endpoint not configured, allowing signal",
			zap.String("signal_id", signalID),
		)
		result.Error = "kairos endpoint not configured"
		return result
	}

	// Get the active policy
	policy := e.policyRegistry.GetPolicy(e.defaultPolicy)
	if policy == nil {
		e.logger.Warn("default policy not found",
			zap.String("policy_id", e.defaultPolicy),
		)
		result.Error = "default policy not found"
		if !e.failOpen {
			result.Decision = "deny"
		}
		return result
	}

	if !policy.IsEnabled() {
		e.logger.Debug("policy is disabled",
			zap.String("policy_id", policy.ID),
		)
		return result
	}

	// Build request for Kairos
	policyReq := &PolicyRequest{
		SignalID:   signalID,
		TraceID:    traceID,
		Layer:      layer,
		Category:   category,
		RuleID:     ruleID,
		RuleName:   ruleName,
		Confidence: confidence,
		Data:       data,
		Timestamp:  time.Now().Unix(),
	}

	// Call Kairos
	policyResp, err := e.client.EvaluatePolicy(ctx, policyReq)
	if err != nil {
		e.logger.Warn("kairos evaluation failed",
			zap.String("signal_id", signalID),
			zap.String("rule_id", ruleID),
			zap.Error(err),
		)
		result.Error = err.Error()

		// Fail-open: allow the signal if Kairos is unreachable
		if e.failOpen {
			result.Decision = "allow"
			result.Reasoning = "Kairos unavailable, failing open"
		} else {
			result.Decision = "review"
			result.Reasoning = "Kairos evaluation failed"
		}
		return result
	}

	// Parse Kairos response
	result.Decision = policyResp.Decision
	result.Confidence = policyResp.Confidence
	result.Reasoning = policyResp.Reasoning
	result.RecommendedAction = policyResp.RecommendedAction
	result.KairosPolicyVersion = policyResp.KairosPolicyVersion
	result.PolicyRuleTriggered = policyResp.PolicyRuleTriggered
	result.ProcessingTimeMs = policyResp.ProcessingTimeMs

	e.logger.Info("kairos evaluation completed",
		zap.String("signal_id", signalID),
		zap.String("rule_id", ruleID),
		zap.String("decision", result.Decision),
		zap.Float64("confidence", result.Confidence),
		zap.String("recommended_action", result.RecommendedAction),
	)

	return result
}

// IsEnabled returns whether the evaluator is enabled.
func (e *Evaluator) IsEnabled() bool {
	return e.enabled
}

// SetEnabled enables or disables the evaluator.
func (e *Evaluator) SetEnabled(enabled bool) {
	e.enabled = enabled
	e.logger.Info("evaluator status changed",
		zap.Bool("enabled", enabled),
	)
}

// SetDefaultPolicy sets the default policy for evaluation.
func (e *Evaluator) SetDefaultPolicy(policyID string) error {
	policy := e.policyRegistry.GetPolicy(policyID)
	if policy == nil {
		return fmt.Errorf("policy not found: %s", policyID)
	}

	e.defaultPolicy = policyID
	e.logger.Info("default policy changed",
		zap.String("policy_id", policyID),
	)
	return nil
}

// GetDefaultPolicy returns the ID of the default policy.
func (e *Evaluator) GetDefaultPolicy() string {
	return e.defaultPolicy
}

// Health checks the health of Kairos endpoint.
func (e *Evaluator) Health(ctx context.Context) error {
	if !e.enabled {
		return fmt.Errorf("evaluator is disabled")
	}

	return e.client.Health(ctx)
}
