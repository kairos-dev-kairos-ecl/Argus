package notify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/argusxdr/argus/internal/alert"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SuppressionRule represents a rule that suppresses alerts.
type SuppressionRule struct {
	RuleID         string     // Unique ID
	Name           string     // Human-readable name
	Description    string     // Description
	Enabled        bool       // Is this rule active?
	StartAt        *time.Time // Time-based: start of suppression window
	EndAt          *time.Time // Time-based: end of suppression window
	PatternType    string     // Pattern-based: "acknowledged_recently" or similar
	PatternConfig  map[string]interface{} // Pattern-specific config
	SuppressedCount int64      // Metrics: count of alerts suppressed by this rule
}

// SuppressionEvaluator evaluates suppression rules to determine if alerts should be suppressed.
type SuppressionEvaluator struct {
	mu                 sync.RWMutex
	rules              map[string]*SuppressionRule
	db                 interface{} // Database connection (postgres or similar) for loading rules
	logger             *zap.Logger
	stopCh             chan struct{}
	suppressionMetrics map[string]int64 // rule_id -> suppressed_count
}

// NewSuppressionEvaluator creates a new suppression evaluator.
func NewSuppressionEvaluator(db interface{}, logger *zap.Logger) (*SuppressionEvaluator, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	se := &SuppressionEvaluator{
		rules:              make(map[string]*SuppressionRule),
		db:                 db,
		logger:             logger,
		stopCh:             make(chan struct{}),
		suppressionMetrics: make(map[string]int64),
	}

	// Load initial rules from database
	if err := se.reloadRules(context.Background()); err != nil {
		logger.Warn("failed to load suppression rules", zap.Error(err))
		// Continue anyway - will try again on next reload
	}

	// Start hot-reload monitor
	go se.hotReloadMonitor()

	return se, nil
}

// IsSuppressed checks if an alert should be suppressed based on active rules.
func (se *SuppressionEvaluator) IsSuppressed(ctx context.Context, a *alert.Alert) (bool, string, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	now := time.Now()

	for ruleID, rule := range se.rules {
		if !rule.Enabled {
			continue
		}

		// Check time-based suppression
		if se.checkTimeBasedSuppression(rule, now) {
			se.logger.Debug("alert suppressed by time-based rule",
				zap.String("rule_id", ruleID),
				zap.String("rule_name", rule.Name))
			se.suppressionMetrics[ruleID]++
			return true, ruleID, nil
		}

		// Check pattern-based suppression
		if se.checkPatternBasedSuppression(ctx, rule, a) {
			se.logger.Debug("alert suppressed by pattern-based rule",
				zap.String("rule_id", ruleID),
				zap.String("rule_name", rule.Name))
			se.suppressionMetrics[ruleID]++
			return true, ruleID, nil
		}
	}

	return false, "", nil
}

// checkTimeBasedSuppression checks if an alert falls within a time-based suppression window.
func (se *SuppressionEvaluator) checkTimeBasedSuppression(rule *SuppressionRule, now time.Time) bool {
	if rule.StartAt == nil || rule.EndAt == nil {
		return false
	}

	return !now.Before(*rule.StartAt) && now.Before(*rule.EndAt)
}

// checkPatternBasedSuppression checks if an alert matches a pattern-based suppression rule.
func (se *SuppressionEvaluator) checkPatternBasedSuppression(ctx context.Context, rule *SuppressionRule, a *alert.Alert) bool {
	switch rule.PatternType {
	case "acknowledged_recently":
		// Suppress if this alert's incident was acknowledged in the last N minutes
		if a.IncidentID == nil {
			return false
		}

		lookback, ok := rule.PatternConfig["lookback_minutes"].(float64)
		if !ok {
			lookback = 60 // Default 60 minutes
		}

		// Query incidents table for this incident's status and last acknowledgment time
		// If incident was acknowledged within lookback period, suppress
		return se.wasIncidentAcknowledgedRecently(ctx, *a.IncidentID, time.Duration(lookback)*time.Minute)

	case "rule_acknowledged":
		// Suppress if the rule that triggered this alert was recently acknowledged for another alert
		lookback, ok := rule.PatternConfig["lookback_minutes"].(float64)
		if !ok {
			lookback = 60
		}
		return se.wasRuleAcknowledgedRecently(ctx, a.RuleID, time.Duration(lookback)*time.Minute)

	default:
		return false
	}
}

// wasIncidentAcknowledgedRecently checks if an incident was acknowledged recently.
func (se *SuppressionEvaluator) wasIncidentAcknowledgedRecently(ctx context.Context, incidentID uuid.UUID, lookback time.Duration) bool {
	// Query PostgreSQL incidents table
	// Check if status = 'acknowledged' AND acknowledged_at > now() - lookback
	// This is a simplified check - in production, you'd use the IncidentService

	// For now, return false to avoid over-suppression
	// Real implementation would query incidents table
	_ = incidentID
	_ = lookback
	return false
}

// wasRuleAcknowledgedRecently checks if a rule was recently acknowledged in another alert.
func (se *SuppressionEvaluator) wasRuleAcknowledgedRecently(ctx context.Context, ruleID uuid.UUID, lookback time.Duration) bool {
	// Query PostgreSQL alerts and incidents tables
	// Check if any alert for this rule_id was promoted to an incident that was acknowledged
	// within lookback period

	// For now, return false
	// Real implementation would query alerts -> incidents table
	_ = ruleID
	_ = lookback
	return false
}

// reloadRules loads suppression rules from the database.
func (se *SuppressionEvaluator) reloadRules(ctx context.Context) error {
	// Query suppression_rules table from PostgreSQL
	// This would typically use se.pg to query the database
	// For now, we'll create a placeholder structure

	se.mu.Lock()
	defer se.mu.Unlock()

	// In a real implementation, this would query the database:
	// rows, err := se.pg.Query(ctx, "SELECT rule_id, name, description, enabled, start_at, end_at, pattern_type, pattern_config FROM suppression_rules WHERE enabled = true")
	// Then populate se.rules from the result set

	// Placeholder: rules start empty and will be populated by database queries
	se.logger.Debug("suppression rules reloaded", zap.Int("count", len(se.rules)))
	return nil
}

// hotReloadMonitor periodically reloads suppression rules from the database.
func (se *SuppressionEvaluator) hotReloadMonitor() {
	ticker := time.NewTicker(5 * time.Minute) // Reload every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := se.reloadRules(context.Background()); err != nil {
				se.logger.Warn("failed to reload suppression rules", zap.Error(err))
			}
		case <-se.stopCh:
			return
		}
	}
}

// GetMetrics returns suppression metrics (suppressed alert counts per rule).
func (se *SuppressionEvaluator) GetMetrics() map[string]int64 {
	se.mu.RLock()
	defer se.mu.RUnlock()

	metrics := make(map[string]int64)
	for ruleID, count := range se.suppressionMetrics {
		metrics[ruleID] = count
	}
	return metrics
}

// Close stops the hot-reload monitor and cleans up resources.
func (se *SuppressionEvaluator) Close() {
	close(se.stopCh)
}

// ResetMetrics resets suppression metrics (for testing).
func (se *SuppressionEvaluator) ResetMetrics() {
	se.mu.Lock()
	defer se.mu.Unlock()

	se.suppressionMetrics = make(map[string]int64)
}
