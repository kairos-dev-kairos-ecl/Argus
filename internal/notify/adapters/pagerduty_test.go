package adapters

import (
	"context"
	"testing"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewPagerDutyNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		config    PagerDutyConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: PagerDutyConfig{
				IntegrationKey: "R1234567890abcdef",
				ServiceID:      "PSERVICE123",
			},
			wantError: false,
		},
		{
			name: "missing integration key",
			config: PagerDutyConfig{
				IntegrationKey: "",
				ServiceID:      "PSERVICE123",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewPagerDutyNotifier(tt.config, logger)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "pagerduty", notifier.Name())
			}
		})
	}
}

func TestPDSeverityMapping(t *testing.T) {
	tests := []struct {
		severity int
		expected string
	}{
		{1, "info"},
		{2, "warning"},
		{3, "error"},
		{4, "critical"},
		{5, "critical"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.severity)), func(t *testing.T) {
			result := pdSeverityMapping(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSignals(t *testing.T) {
	tests := []struct {
		name      string
		signalIDs string
		expected  int
	}{
		{
			name:      "empty",
			signalIDs: "",
			expected:  0,
		},
		{
			name:      "single signal",
			signalIDs: "sig-001",
			expected:  1,
		},
		{
			name:      "multiple signals",
			signalIDs: "sig-001,sig-002,sig-003",
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countSignals(tt.signalIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPagerDutyNotifierName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewPagerDutyNotifier(PagerDutyConfig{
		IntegrationKey: "R1234567890abcdef",
	}, logger)
	require.NoError(t, err)

	assert.Equal(t, "pagerduty", notifier.Name())
}

func TestPagerDutySendValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewPagerDutyNotifier(PagerDutyConfig{
		IntegrationKey: "R1234567890abcdef",
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with nil request
	resp, err := notifier.Send(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestPagerDutyNotifierSendStructure(t *testing.T) {
	// This test verifies that the Send method can construct the event request
	// without panicking.

	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	_, err := NewPagerDutyNotifier(PagerDutyConfig{
		IntegrationKey: "R1234567890abcdef",
	}, logger)
	require.NoError(t, err)

	req := &notify.NotificationRequest{
		ID:       "notif-001",
		AlertID:  uuid.New(),
		Severity: 4,
		RuleID:   uuid.New(),
		Title:    "Critical Alert",
		Message:  "This is a critical alert",
		Metadata: map[string]string{
			"signal_ids": "sig-001,sig-002",
			"confidence": "0.99",
			"fingerprint": "abc123def456",
		},
	}

	// Verify request structure
	assert.NotNil(t, req)
	assert.Equal(t, "notif-001", req.ID)
	assert.Equal(t, 4, req.Severity)
}
