package notify

import (
	"context"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/alert"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestNewSuppressionEvaluator(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	t.Run("nil db", func(t *testing.T) {
		se, err := NewSuppressionEvaluator(nil, logger)
		assert.Error(t, err)
		assert.Nil(t, se)
		assert.Contains(t, err.Error(), "database connection is required")
	})

	// Note: Creating with real database would require test database
	// So we skip the success case in unit tests
}

func TestCheckTimeBasedSuppression(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		startAt   *time.Time
		endAt     *time.Time
		now       time.Time
		expect    bool
	}{
		{
			name:    "no time window",
			startAt: nil,
			endAt:   nil,
			now:     time.Now(),
			expect:  false,
		},
		{
			name:    "before window",
			startAt: ptrTime(time.Now().Add(1 * time.Hour)),
			endAt:   ptrTime(time.Now().Add(2 * time.Hour)),
			now:     time.Now(),
			expect:  false,
		},
		{
			name:    "after window",
			startAt: ptrTime(time.Now().Add(-2 * time.Hour)),
			endAt:   ptrTime(time.Now().Add(-1 * time.Hour)),
			now:     time.Now(),
			expect:  false,
		},
		{
			name:    "within window",
			startAt: ptrTime(time.Now().Add(-1 * time.Hour)),
			endAt:   ptrTime(time.Now().Add(1 * time.Hour)),
			now:     time.Now(),
			expect:  true,
		},
		{
			name:    "at window start",
			startAt: ptrTime(time.Now()),
			endAt:   ptrTime(time.Now().Add(1 * time.Hour)),
			now:     time.Now(),
			expect:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &SuppressionRule{
				RuleID: "test-rule",
				Name:   "Test Rule",
				StartAt: tt.startAt,
				EndAt:   tt.endAt,
			}

			se := &SuppressionEvaluator{
				logger: logger,
			}

			result := se.checkTimeBasedSuppression(rule, tt.now)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestCheckPatternBasedSuppression(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	se := &SuppressionEvaluator{
		logger: logger,
	}

	tests := []struct {
		name       string
		patternType string
		config     map[string]interface{}
		expect     bool
	}{
		{
			name:        "acknowledged_recently with default lookback",
			patternType: "acknowledged_recently",
			config:      make(map[string]interface{}),
			expect:      false, // No incident to check
		},
		{
			name:        "rule_acknowledged with custom lookback",
			patternType: "rule_acknowledged",
			config: map[string]interface{}{
				"lookback_minutes": float64(30),
			},
			expect: false,
		},
		{
			name:        "unknown pattern type",
			patternType: "unknown",
			config:      make(map[string]interface{}),
			expect:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &SuppressionRule{
				RuleID:      "test-rule",
				Name:        "Test Rule",
				PatternType: tt.patternType,
				PatternConfig: tt.config,
			}

			testAlert := &alert.Alert{
				AlertID: uuid.New(),
				RuleID:  uuid.New(),
			}

			result := se.checkPatternBasedSuppression(context.Background(), rule, testAlert)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestGetMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	se := &SuppressionEvaluator{
		logger:             logger,
		suppressionMetrics: map[string]int64{"rule1": 5, "rule2": 10},
	}

	metrics := se.GetMetrics()
	assert.Equal(t, int64(5), metrics["rule1"])
	assert.Equal(t, int64(10), metrics["rule2"])
}

func TestResetMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	se := &SuppressionEvaluator{
		logger:             logger,
		suppressionMetrics: map[string]int64{"rule1": 5, "rule2": 10},
	}

	se.ResetMetrics()
	metrics := se.GetMetrics()
	assert.Empty(t, metrics)
}

func TestClose(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	se := &SuppressionEvaluator{
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// Should not panic
	se.Close()

	// Verify stopCh is closed (reading from it should immediately return)
	_, ok := <-se.stopCh
	assert.False(t, ok, "stopCh should be closed")
}

func TestIsSuppressedNoRules(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	se := &SuppressionEvaluator{
		logger:             logger,
		rules:              make(map[string]*SuppressionRule),
		suppressionMetrics: make(map[string]int64),
	}

	testAlert := &alert.Alert{
		AlertID: uuid.New(),
		RuleID:  uuid.New(),
	}

	suppressed, ruleID, err := se.IsSuppressed(context.Background(), testAlert)
	assert.NoError(t, err)
	assert.False(t, suppressed)
	assert.Empty(t, ruleID)
}

func TestIsSuppressedWithDisabledRule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	now := time.Now()
	se := &SuppressionEvaluator{
		logger: logger,
		rules: map[string]*SuppressionRule{
			"disabled-rule": {
				RuleID:  "disabled-rule",
				Name:    "Disabled Rule",
				Enabled: false, // Disabled
				StartAt: &now,
				EndAt:   ptrTime(now.Add(1 * time.Hour)),
			},
		},
		suppressionMetrics: make(map[string]int64),
	}

	testAlert := &alert.Alert{
		AlertID: uuid.New(),
		RuleID:  uuid.New(),
	}

	suppressed, ruleID, err := se.IsSuppressed(context.Background(), testAlert)
	assert.NoError(t, err)
	assert.False(t, suppressed)
	assert.Empty(t, ruleID)
}

func TestIsSuppressedWithTimeBasedRule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	now := time.Now()
	se := &SuppressionEvaluator{
		logger: logger,
		rules: map[string]*SuppressionRule{
			"time-rule": {
				RuleID:  "time-rule",
				Name:    "Time-Based Rule",
				Enabled: true,
				StartAt: ptrTime(now.Add(-1 * time.Hour)),
				EndAt:   ptrTime(now.Add(1 * time.Hour)),
			},
		},
		suppressionMetrics: make(map[string]int64),
	}

	testAlert := &alert.Alert{
		AlertID: uuid.New(),
		RuleID:  uuid.New(),
	}

	suppressed, ruleID, err := se.IsSuppressed(context.Background(), testAlert)
	assert.NoError(t, err)
	assert.True(t, suppressed)
	assert.Equal(t, "time-rule", ruleID)

	// Verify metrics incremented
	metrics := se.GetMetrics()
	assert.Equal(t, int64(1), metrics["time-rule"])
}

// Helper function to create time pointers
func ptrTime(t time.Time) *time.Time {
	return &t
}
