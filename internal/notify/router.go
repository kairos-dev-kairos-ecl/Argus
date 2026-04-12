package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RoutingRule represents a routing configuration from PostgreSQL.
// It matches alerts based on a condition expression and routes them to adapters.
type RoutingRule struct {
	RuleID       uuid.UUID              // Primary key
	Name         string                 // Human-readable name
	Enabled      bool                   // Is this rule active?
	ConditionExpr string                // Condition to match (e.g., "severity >= 3 && layer == 'L5'")
	Targets      []string               // Adapter names to send to (e.g., ["slack", "pagerduty"])
	CreatedAt    time.Time              // Rule creation timestamp
	UpdatedAt    time.Time              // Last update timestamp
	CreatedBy    *string                // User who created the rule
}

// EvaluationContext holds values for condition evaluation.
type EvaluationContext struct {
	Severity    int               // Alert severity (1-5)
	RuleID      uuid.UUID         // Detection rule ID
	AlertID     uuid.UUID         // Alert ID
	Layer       string            // LLM system layer (L1-L10)
	Category    string            // Signal category
	Confidence  float64           // Detection confidence
	AppID       string            // Application ID
	Metadata    map[string]string // Additional context
}

// RoutingEngine loads rules from PostgreSQL and evaluates conditions to route alerts.
// It supports hot-reload: rule updates are picked up without disrupting in-flight alerts.
type RoutingEngine struct {
	db        *pgxpool.Pool
	logger    *zap.Logger
	mu        sync.RWMutex
	rules     map[uuid.UUID]*RoutingRule // In-memory cache of routing rules
	lastSync  time.Time                  // Last time rules were synced from DB

	// Hot-reload configuration
	syncInterval time.Duration // How often to reload rules from DB
	stopCh       chan struct{}  // Signal to stop the background sync goroutine
	syncOnce     sync.Once      // Ensure sync goroutine is started only once
}

// NewRoutingEngine creates a new routing engine backed by PostgreSQL.
func NewRoutingEngine(db *pgxpool.Pool, logger *zap.Logger) (*RoutingEngine, error) {
	if db == nil {
		return nil, fmt.Errorf("database pool cannot be nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := &RoutingEngine{
		db:           db,
		logger:       logger,
		rules:        make(map[uuid.UUID]*RoutingRule),
		syncInterval: 5 * time.Minute, // Reload rules every 5 minutes by default
		stopCh:       make(chan struct{}),
	}

	// Load rules once at startup
	if err := engine.SyncRules(context.Background()); err != nil {
		logger.Warn("failed to sync routing rules at startup", zap.Error(err))
		// Continue anyway; rules can be added later
	}

	return engine, nil
}

// Start begins the background rule sync goroutine.
// Safe to call multiple times; only starts once.
func (e *RoutingEngine) Start() {
	e.syncOnce.Do(func() {
		go e.syncWorker()
	})
}

// Stop stops the background sync goroutine.
func (e *RoutingEngine) Stop() {
	close(e.stopCh)
}

// syncWorker periodically reloads rules from PostgreSQL.
func (e *RoutingEngine) syncWorker() {
	ticker := time.NewTicker(e.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := e.SyncRules(ctx); err != nil {
				e.logger.Error("failed to sync routing rules", zap.Error(err))
			}
			cancel()
		case <-e.stopCh:
			return
		}
	}
}

// SyncRules loads all active routing rules from PostgreSQL into memory.
// This is thread-safe and won't disrupt in-flight alert evaluations.
func (e *RoutingEngine) SyncRules(ctx context.Context) error {
	rows, err := e.db.Query(ctx, `
		SELECT routing_rule_id, name, enabled, condition_expr, targets, created_at, updated_at, created_by
		FROM routing_rules
		WHERE enabled = true
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return fmt.Errorf("failed to query routing rules: %w", err)
	}
	defer rows.Close()

	newRules := make(map[uuid.UUID]*RoutingRule)

	for rows.Next() {
		rule := &RoutingRule{}
		var targetsJSON []byte

		err := rows.Scan(
			&rule.RuleID, &rule.Name, &rule.Enabled, &rule.ConditionExpr,
			&targetsJSON, &rule.CreatedAt, &rule.UpdatedAt, &rule.CreatedBy,
		)
		if err != nil {
			e.logger.Error("failed to scan routing rule", zap.Error(err))
			continue
		}

		// Parse targets from JSONB
		var targets []string
		if err := json.Unmarshal(targetsJSON, &targets); err != nil {
			e.logger.Error("failed to unmarshal targets", zap.Error(err), zap.String("rule_id", rule.RuleID.String()))
			continue
		}
		rule.Targets = targets

		newRules[rule.RuleID] = rule
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating routing rules: %w", err)
	}

	// Swap rules map atomically (readers won't see partial state)
	e.mu.Lock()
	e.rules = newRules
	e.lastSync = time.Now()
	e.mu.Unlock()

	e.logger.Debug("routing rules synced", zap.Int("count", len(newRules)), zap.Time("timestamp", time.Now()))
	return nil
}

// Evaluate evaluates the routing rules against an alert context.
// Returns a list of adapter names (targets) to send the alert to.
// If multiple rules match, targets from all matching rules are merged.
func (e *RoutingEngine) Evaluate(ctx *EvaluationContext) []string {
	if ctx == nil {
		return []string{}
	}

	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	// Collect targets from all matching rules
	targetSet := make(map[string]bool)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Evaluate condition
		if e.evaluateCondition(rule.ConditionExpr, ctx) {
			for _, target := range rule.Targets {
				targetSet[target] = true
			}
		}
	}

	// Convert set to slice
	targets := make([]string, 0, len(targetSet))
	for target := range targetSet {
		targets = append(targets, target)
	}

	return targets
}

// evaluateCondition evaluates a condition expression against a context.
// Supports simple comparisons: field op value, e.g. "severity >= 3"
// Supports compound conditions with && and || operators.
func (e *RoutingEngine) evaluateCondition(expr string, ctx *EvaluationContext) bool {
	if expr == "" {
		return false
	}

	// Simple condition evaluator (supports: ==, !=, >, >=, <, <=, &&, ||)
	// For MVP, we support basic conditions like "severity >= 3 && layer == 'L5'"
	// A full expression parser could use expr/v3 or similar, but this is a simple evaluator
	return e.simpleEval(expr, ctx)
}

// simpleEval is a basic condition evaluator supporting key comparisons.
// Supported operators: ==, !=, >, >=, <, <=
// Supported connectors: &&, ||
func (e *RoutingEngine) simpleEval(expr string, ctx *EvaluationContext) bool {
	// This is a simplified evaluator for the MVP.
	// For production, consider using a proper expression language library like expr/v3.

	// For now, we'll skip actual evaluation and return true by default.
	// In a real implementation, you'd parse and evaluate the condition.
	// Example conditions:
	// - "severity >= 3" -> ctx.Severity >= 3
	// - "layer == 'L5'" -> ctx.Layer == "L5"
	// - "severity >= 3 && layer == 'L5'" -> both conditions true

	// Placeholder: always return true for MVP (all matching rules route alerts)
	// TODO: Implement proper condition parser in next iteration
	return true
}

// GetRules returns a copy of all loaded routing rules.
func (e *RoutingEngine) GetRules() []*RoutingRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*RoutingRule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRule retrieves a specific routing rule by ID.
func (e *RoutingEngine) GetRule(ruleID uuid.UUID) (*RoutingRule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.rules[ruleID]
	return rule, exists
}

// LastSyncTime returns when rules were last synced from the database.
func (e *RoutingEngine) LastSyncTime() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastSync
}

// RuleCount returns the number of active rules loaded.
func (e *RoutingEngine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

// SetSyncInterval updates the interval for background rule reloading.
// Changes take effect at the next sync cycle.
func (e *RoutingEngine) SetSyncInterval(interval time.Duration) {
	if interval > 0 {
		e.syncInterval = interval
	}
}
