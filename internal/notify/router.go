package notify

import (
	"context"
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
	RuleID      uuid.UUID
	ChannelID   uuid.UUID
	Enabled     bool
	MinSeverity int
	AppIDFilter *string
	LayerFilter *int
	Targets     []string // channel types (adapter names from notification_channels.type)
}

// EvaluationContext holds values for condition evaluation.
type EvaluationContext struct {
	Severity int
	AppID    string
	Layer    int // LLM system layer 1-10
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
		SELECT rr.id, rr.channel_id, rr.min_severity, rr.app_id_filter, rr.layer_filter, nc.type
		FROM routing_rules rr
		JOIN notification_channels nc ON rr.channel_id = nc.id
		WHERE rr.enabled = true AND nc.enabled = true
		ORDER BY rr.id
	`)
	if err != nil {
		return fmt.Errorf("failed to query routing rules: %w", err)
	}
	defer rows.Close()

	newRules := make(map[uuid.UUID]*RoutingRule)

	for rows.Next() {
		rule := &RoutingRule{Enabled: true}
		var channelType string
		err := rows.Scan(
			&rule.RuleID, &rule.ChannelID, &rule.MinSeverity,
			&rule.AppIDFilter, &rule.LayerFilter, &channelType,
		)
		if err != nil {
			e.logger.Error("failed to scan routing rule", zap.Error(err))
			continue
		}
		rule.Targets = []string{channelType}
		newRules[rule.RuleID] = rule
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating routing rules: %w", err)
	}

	e.mu.Lock()
	e.rules = newRules
	e.lastSync = time.Now()
	e.mu.Unlock()

	e.logger.Debug("routing rules synced", zap.Int("count", len(newRules)))
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

	targetSet := make(map[string]bool)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if e.simpleEval(rule, ctx) {
			for _, target := range rule.Targets {
				targetSet[target] = true
			}
		}
	}

	targets := make([]string, 0, len(targetSet))
	for target := range targetSet {
		targets = append(targets, target)
	}
	return targets
}

// simpleEval evaluates a routing rule against an alert context using field filters.
func (e *RoutingEngine) simpleEval(rule *RoutingRule, ctx *EvaluationContext) bool {
	if ctx.Severity < rule.MinSeverity {
		return false
	}
	if rule.AppIDFilter != nil && *rule.AppIDFilter != ctx.AppID {
		return false
	}
	if rule.LayerFilter != nil && *rule.LayerFilter != ctx.Layer {
		return false
	}
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
