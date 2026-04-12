package kairos

import (
	"fmt"

	"go.uber.org/zap"
)

// Policy represents a Kairos ECL policy configuration.
type Policy struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Rules    []Rule `json:"rules"`
	logger   *zap.Logger
}

// Rule represents a single policy rule.
type Rule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Condition  string `json:"condition"` // ECL expression
	Action     string `json:"action"`    // allow, deny, review
	Priority   int    `json:"priority"`
	Confidence float64 `json:"confidence"` // Expected confidence level
}

// NewPolicy creates a new policy with validation.
func NewPolicy(id, version, name string, enabled bool, logger *zap.Logger) *Policy {
	return &Policy{
		ID:      id,
		Version: version,
		Name:    name,
		Enabled: enabled,
		Rules:   make([]Rule, 0),
		logger:  logger,
	}
}

// AddRule adds a rule to the policy after validation.
func (p *Policy) AddRule(rule Rule) error {
	if err := validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	p.Rules = append(p.Rules, rule)
	p.logger.Debug("rule added to policy",
		zap.String("policy_id", p.ID),
		zap.String("rule_id", rule.ID),
		zap.String("rule_name", rule.Name),
	)

	return nil
}

// Validate checks the policy structure and content.
func (p *Policy) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("policy id cannot be empty")
	}

	if p.Version == "" {
		return fmt.Errorf("policy version cannot be empty")
	}

	if len(p.Rules) == 0 && p.Enabled {
		return fmt.Errorf("policy has no rules but is enabled")
	}

	// Validate all rules
	for i, rule := range p.Rules {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("rule %d invalid: %w", i, err)
		}
	}

	return nil
}

// validateRule validates a single rule.
func validateRule(rule Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule id cannot be empty")
	}

	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if rule.Condition == "" {
		return fmt.Errorf("rule condition cannot be empty")
	}

	validActions := map[string]bool{"allow": true, "deny": true, "review": true}
	if !validActions[rule.Action] {
		return fmt.Errorf("invalid action: %s (must be allow, deny, or review)", rule.Action)
	}

	if rule.Confidence < 0 || rule.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1, got %f", rule.Confidence)
	}

	return nil
}

// GetRule returns a rule by ID.
func (p *Policy) GetRule(ruleID string) *Rule {
	for _, rule := range p.Rules {
		if rule.ID == ruleID {
			return &rule
		}
	}
	return nil
}

// IsEnabled returns whether the policy is enabled.
func (p *Policy) IsEnabled() bool {
	return p.Enabled
}

// RuleCount returns the number of rules in the policy.
func (p *Policy) RuleCount() int {
	return len(p.Rules)
}

// PolicyRegistry manages multiple policies.
type PolicyRegistry struct {
	policies map[string]*Policy
	logger   *zap.Logger
}

// NewPolicyRegistry creates a new policy registry.
func NewPolicyRegistry(logger *zap.Logger) *PolicyRegistry {
	return &PolicyRegistry{
		policies: make(map[string]*Policy),
		logger:   logger,
	}
}

// RegisterPolicy registers a policy in the registry.
func (pr *PolicyRegistry) RegisterPolicy(policy *Policy) error {
	if err := policy.Validate(); err != nil {
		pr.logger.Error("invalid policy",
			zap.String("policy_id", policy.ID),
			zap.Error(err),
		)
		return fmt.Errorf("invalid policy: %w", err)
	}

	pr.policies[policy.ID] = policy
	pr.logger.Info("policy registered",
		zap.String("policy_id", policy.ID),
		zap.String("version", policy.Version),
		zap.Int("rule_count", policy.RuleCount()),
	)

	return nil
}

// GetPolicy retrieves a policy by ID.
func (pr *PolicyRegistry) GetPolicy(policyID string) *Policy {
	return pr.policies[policyID]
}

// ListPolicies returns all registered policies.
func (pr *PolicyRegistry) ListPolicies() []*Policy {
	policies := make([]*Policy, 0, len(pr.policies))
	for _, policy := range pr.policies {
		policies = append(policies, policy)
	}
	return policies
}

// RemovePolicy removes a policy from the registry.
func (pr *PolicyRegistry) RemovePolicy(policyID string) bool {
	if _, exists := pr.policies[policyID]; exists {
		delete(pr.policies, policyID)
		pr.logger.Info("policy removed",
			zap.String("policy_id", policyID),
		)
		return true
	}
	return false
}

// PolicyCount returns the number of registered policies.
func (pr *PolicyRegistry) PolicyCount() int {
	return len(pr.policies)
}
