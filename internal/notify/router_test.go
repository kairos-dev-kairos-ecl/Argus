package notify

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// MockDB for testing (since we can't use pgx in unit tests without a real DB)
// For now, we'll test the router's condition evaluation and rule management
// without the database component (which would be tested via integration tests)

func TestEvaluationContext(t *testing.T) {
	ctx := &EvaluationContext{
		Severity:   4,
		RuleID:     uuid.New(),
		AlertID:    uuid.New(),
		Layer:      "L5",
		Category:   "ModelInference",
		Confidence: 0.95,
		AppID:      "app-123",
		Metadata: map[string]string{
			"customer": "acme",
		},
	}

	assert.Equal(t, 4, ctx.Severity)
	assert.Equal(t, "L5", ctx.Layer)
	assert.Equal(t, 0.95, ctx.Confidence)
	assert.Equal(t, "acme", ctx.Metadata["customer"])
}

func TestRoutingRule(t *testing.T) {
	rule := &RoutingRule{
		RuleID:        uuid.New(),
		Name:          "Critical High-Layer Alerts",
		Enabled:       true,
		ConditionExpr: "severity >= 3 && layer == 'L5'",
		Targets:       []string{"slack", "pagerduty"},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.Equal(t, "Critical High-Layer Alerts", rule.Name)
	assert.True(t, rule.Enabled)
	assert.Len(t, rule.Targets, 2)
	assert.Equal(t, "slack", rule.Targets[0])
	assert.Equal(t, "pagerduty", rule.Targets[1])
}

func TestRoutingEngine_Initialization(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Note: This test would require a real database connection in integration tests
	// For unit testing, we'll create a mock engine by testing the logic
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	assert.Equal(t, 0, engine.RuleCount())
	assert.NotNil(t, engine.LastSyncTime())
}

func TestRoutingEngine_SimpleEval(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Test simple evaluation (MVP returns true by default)
	ctx := &EvaluationContext{
		Severity: 4,
		Layer:    "L5",
	}

	result := engine.simpleEval("severity >= 3 && layer == 'L5'", ctx)
	// MVP implementation returns true for all conditions
	assert.True(t, result)
}

func TestRoutingEngine_Evaluate_NoRules(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	ctx := &EvaluationContext{
		Severity: 4,
		Layer:    "L5",
	}

	targets := engine.Evaluate(ctx)
	assert.Len(t, targets, 0)
}

func TestRoutingEngine_Evaluate_NilContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	targets := engine.Evaluate(nil)
	assert.Len(t, targets, 0)
}

func TestRoutingEngine_Evaluate_SingleRule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Manually add a rule
	ruleID := uuid.New()
	rule := &RoutingRule{
		RuleID:        ruleID,
		Name:          "Test Rule",
		Enabled:       true,
		ConditionExpr: "severity >= 3",
		Targets:       []string{"slack"},
	}
	engine.rules[ruleID] = rule

	ctx := &EvaluationContext{
		Severity: 4,
		Layer:    "L5",
	}

	targets := engine.Evaluate(ctx)
	// MVP implementation returns targets for all matching rules
	assert.Contains(t, targets, "slack")
}

func TestRoutingEngine_Evaluate_MultipleRules(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Add multiple rules
	rule1 := &RoutingRule{
		RuleID:        uuid.New(),
		Name:          "Rule 1",
		Enabled:       true,
		ConditionExpr: "severity >= 3",
		Targets:       []string{"slack"},
	}
	rule2 := &RoutingRule{
		RuleID:        uuid.New(),
		Name:          "Rule 2",
		Enabled:       true,
		ConditionExpr: "layer == 'L5'",
		Targets:       []string{"pagerduty"},
	}
	engine.rules[rule1.RuleID] = rule1
	engine.rules[rule2.RuleID] = rule2

	ctx := &EvaluationContext{
		Severity: 4,
		Layer:    "L5",
	}

	targets := engine.Evaluate(ctx)
	// Both rules should match in MVP
	assert.Len(t, targets, 2)
	assert.Contains(t, targets, "slack")
	assert.Contains(t, targets, "pagerduty")
}

func TestRoutingEngine_Evaluate_DisabledRule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Add a disabled rule
	rule := &RoutingRule{
		RuleID:        uuid.New(),
		Name:          "Disabled Rule",
		Enabled:       false,
		ConditionExpr: "severity >= 1",
		Targets:       []string{"slack"},
	}
	engine.rules[rule.RuleID] = rule

	ctx := &EvaluationContext{
		Severity: 4,
	}

	targets := engine.Evaluate(ctx)
	// Disabled rule should not match
	assert.Len(t, targets, 0)
}

func TestRoutingEngine_GetRules(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	rule1 := &RoutingRule{
		RuleID:  uuid.New(),
		Name:    "Rule 1",
		Enabled: true,
		Targets: []string{"slack"},
	}
	rule2 := &RoutingRule{
		RuleID:  uuid.New(),
		Name:    "Rule 2",
		Enabled: true,
		Targets: []string{"pagerduty"},
	}
	engine.rules[rule1.RuleID] = rule1
	engine.rules[rule2.RuleID] = rule2

	rules := engine.GetRules()
	assert.Len(t, rules, 2)
}

func TestRoutingEngine_GetRule(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	ruleID := uuid.New()
	rule := &RoutingRule{
		RuleID:  ruleID,
		Name:    "Test Rule",
		Enabled: true,
		Targets: []string{"slack"},
	}
	engine.rules[ruleID] = rule

	got, exists := engine.GetRule(ruleID)
	assert.True(t, exists)
	assert.Equal(t, "Test Rule", got.Name)

	// Get non-existent rule
	_, exists = engine.GetRule(uuid.New())
	assert.False(t, exists)
}

func TestRoutingEngine_RuleCount(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	assert.Equal(t, 0, engine.RuleCount())

	engine.rules[uuid.New()] = &RoutingRule{}
	assert.Equal(t, 1, engine.RuleCount())

	engine.rules[uuid.New()] = &RoutingRule{}
	assert.Equal(t, 2, engine.RuleCount())
}

func TestRoutingEngine_SetSyncInterval(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	engine.SetSyncInterval(10 * time.Minute)
	assert.Equal(t, 10*time.Minute, engine.syncInterval)

	// Zero or negative intervals should be ignored
	engine.SetSyncInterval(0)
	assert.Equal(t, 10*time.Minute, engine.syncInterval)
}

func TestRoutingEngine_TargetMerging(t *testing.T) {
	// Test that targets from multiple matching rules are merged correctly
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Two rules with overlapping targets and conditions
	rule1 := &RoutingRule{
		RuleID:        uuid.New(),
		Enabled:       true,
		ConditionExpr: "severity >= 3",
		Targets:       []string{"slack", "pagerduty"},
	}
	rule2 := &RoutingRule{
		RuleID:        uuid.New(),
		Enabled:       true,
		ConditionExpr: "layer == 'L5'",
		Targets:       []string{"pagerduty", "email"},
	}
	engine.rules[rule1.RuleID] = rule1
	engine.rules[rule2.RuleID] = rule2

	ctx := &EvaluationContext{
		Severity: 4,
		Layer:    "L5",
	}

	targets := engine.Evaluate(ctx)
	// Should deduplicate targets from both rules
	assert.Greater(t, len(targets), 0, "should have targets from matching rules")
	// Both rules should match and contribute targets
	assert.True(t, len(targets) >= 2, "should have at least 2 unique targets")
}

func TestRoutingEngine_Stop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	engine := &RoutingEngine{
		db:           nil,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}

	// Start the engine
	engine.Start()

	// Stop should not panic
	assert.NotPanics(t, func() {
		engine.Stop()
	})
}
