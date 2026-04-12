package kairos

import (
	"fmt"
)

// Policy defines rules for ECL policy evaluation
type Policy struct {
	Name            string        `json:"name"`
	Version         string        `json:"version"`
	Description     string        `json:"description"`
	Enabled         bool          `json:"enabled"`
	FailOpen        bool          `json:"fail_open"`
	Rules           []PolicyRule  `json:"rules"`
	DefaultDecision string        `json:"default_decision"` // "allow", "deny", "review"
}

// PolicyRule defines a single evaluation rule
type PolicyRule struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Condition       string        `json:"condition"`        // e.g., "severity >= HIGH AND risk_score > 0.7"
	Decision        string        `json:"decision"`         // "allow", "deny", "review"
	Priority        int           `json:"priority"`         // Higher priority evaluated first
	Action          string        `json:"action"`           // "suppress", "escalate", "investigate"
	ReasonTemplate  string        `json:"reason_template"`  // Template for reasoning text
}

// ValidatePolicy checks that a policy is structurally valid
func ValidatePolicy(p *Policy) error {
	if p == nil {
		return fmt.Errorf("policy is nil")
	}

	if p.Name == "" {
		return fmt.Errorf("policy name is empty")
	}

	if p.Version == "" {
		return fmt.Errorf("policy version is empty")
	}

	if len(p.Rules) == 0 {
		return fmt.Errorf("policy has no rules")
	}

	validDecisions := map[string]bool{"allow": true, "deny": true, "review": true}
	if !validDecisions[p.DefaultDecision] {
		return fmt.Errorf("invalid default decision: %s", p.DefaultDecision)
	}

	for i, rule := range p.Rules {
		if rule.ID == "" {
			return fmt.Errorf("rule %d has empty ID", i)
		}

		if !validDecisions[rule.Decision] {
			return fmt.Errorf("rule %s has invalid decision: %s", rule.ID, rule.Decision)
		}

		if rule.Condition == "" {
			return fmt.Errorf("rule %s has empty condition", rule.ID)
		}

		validActions := map[string]bool{"suppress": true, "escalate": true, "investigate": true}
		if !validActions[rule.Action] {
			return fmt.Errorf("rule %s has invalid action: %s", rule.ID, rule.Action)
		}
	}

	return nil
}

// DefaultPolicy returns a minimal valid policy for testing
func DefaultPolicy() *Policy {
	return &Policy{
		Name:            "default",
		Version:         "1.0.0",
		Description:     "Default policy: block on critical severity, review on high risk",
		Enabled:         true,
		FailOpen:        true,
		DefaultDecision: "allow",
		Rules: []PolicyRule{
			{
				ID:             "critical-deny",
				Name:           "Block Critical Signals",
				Condition:      "severity == CRITICAL",
				Decision:       "deny",
				Priority:       100,
				Action:         "escalate",
				ReasonTemplate: "Critical severity signal blocked by policy",
			},
			{
				ID:             "high-risk-review",
				Name:           "Review High Risk Signals",
				Condition:      "risk_score > 0.8",
				Decision:       "review",
				Priority:       90,
				Action:         "investigate",
				ReasonTemplate: "High risk signal (score: {{risk_score}}) requires review",
			},
			{
				ID:             "suspicious-pattern",
				Name:           "Suspicious Patterns",
				Condition:      "category CONTAINS 'injection' OR category CONTAINS 'exfiltration'",
				Decision:       "review",
				Priority:       80,
				Action:         "investigate",
				ReasonTemplate: "Suspicious pattern detected in category: {{category}}",
			},
		},
	}
}

// PolicyConfig holds configuration for Kairos integration
type PolicyConfig struct {
	Enabled      bool          `json:"enabled"`
	Endpoint     string        `json:"endpoint"`       // Kairos HTTP endpoint
	TimeoutMs    int           `json:"timeout_ms"`     // Request timeout
	Policy       *Policy       `json:"policy"`         // Inline policy definition
	PolicyURL    string        `json:"policy_url"`     // URL to fetch policy from
	FailOpen     bool          `json:"fail_open"`      // If true, continue detection if Kairos unavailable
	CacheSize    int           `json:"cache_size"`     // Decision cache size
	CacheTTLSec  int           `json:"cache_ttl_sec"`  // Decision cache TTL in seconds
}

// ValidateConfig checks that a policy config is structurally valid
func ValidateConfig(cfg *PolicyConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if !cfg.Enabled {
		return nil // No validation needed if disabled
	}

	if cfg.Endpoint == "" && cfg.PolicyURL == "" {
		return fmt.Errorf("either endpoint or policy_url must be provided")
	}

	if cfg.TimeoutMs < 100 {
		cfg.TimeoutMs = 100
	}
	if cfg.TimeoutMs > 30000 {
		cfg.TimeoutMs = 30000
	}

	if cfg.CacheSize < 100 {
		cfg.CacheSize = 100
	}
	if cfg.CacheTTLSec < 10 {
		cfg.CacheTTLSec = 10
	}

	return nil
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *PolicyConfig {
	return &PolicyConfig{
		Enabled:     false, // Disabled by default
		Endpoint:    "http://localhost:8888/api/v1/evaluate",
		TimeoutMs:   5000,
		FailOpen:    true,
		CacheSize:   10000,
		CacheTTLSec: 300,
		Policy:      DefaultPolicy(),
	}
}
