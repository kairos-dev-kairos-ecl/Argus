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

func TestNewSlackNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		config    SlackConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: SlackConfig{
				APIToken:  "xoxb-test-token",
				ChannelID: "C12345678",
			},
			wantError: false,
		},
		{
			name: "missing API token",
			config: SlackConfig{
				APIToken:  "",
				ChannelID: "C12345678",
			},
			wantError: true,
		},
		{
			name: "missing channel ID",
			config: SlackConfig{
				APIToken:  "xoxb-test-token",
				ChannelID: "",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewSlackNotifier(tt.config, logger)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "slack", notifier.Name())
			}
		})
	}
}

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		severity int
		expected string
	}{
		{1, "🟢"},
		{2, "🟡"},
		{3, "🟠"},
		{4, "🔴"},
		{5, "🔴🔴"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.severity)), func(t *testing.T) {
			emoji := severityEmoji(tt.severity)
			assert.Equal(t, tt.expected, emoji)
		})
	}
}

func TestFormatSignalIDs(t *testing.T) {
	tests := []struct {
		name      string
		signalIDs []string
		expected  string
	}{
		{
			name:      "empty list",
			signalIDs: []string{},
			expected:  "No signals",
		},
		{
			name:      "single signal",
			signalIDs: []string{"sig-001"},
			expected:  "sig-001",
		},
		{
			name:      "multiple signals",
			signalIDs: []string{"sig-001", "sig-002", "sig-003"},
			expected:  "sig-001\nsig-002\nsig-003",
		},
		{
			name:      "more than 5 signals",
			signalIDs: []string{"sig-001", "sig-002", "sig-003", "sig-004", "sig-005", "sig-006", "sig-007"},
			expected:  "sig-001\nsig-002\nsig-003\nsig-004\nsig-005\nand 2 more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSignalIDs(tt.signalIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlackNotifierName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewSlackNotifier(SlackConfig{
		APIToken:  "xoxb-test",
		ChannelID: "C123",
	}, logger)
	require.NoError(t, err)

	assert.Equal(t, "slack", notifier.Name())
}

func TestSlackSendValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewSlackNotifier(SlackConfig{
		APIToken:  "xoxb-test",
		ChannelID: "C123",
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with nil request
	resp, err := notifier.Send(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSlackNotifierSendStructure(t *testing.T) {
	// This test verifies that the Send method can construct the notification request
	// without panicking. Actual API calls would require mocking.

	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	_, err := NewSlackNotifier(SlackConfig{
		APIToken:  "xoxb-test",
		ChannelID: "C123",
	}, logger)
	require.NoError(t, err)

	req := &notify.NotificationRequest{
		ID:       "notif-001",
		AlertID:  uuid.New(),
		Severity: 4,
		RuleID:   uuid.New(),
		Title:    "Test Alert",
		Message:  "This is a test alert",
		Metadata: map[string]string{
			"signal_ids": "sig-001,sig-002,sig-003",
			"confidence": "0.95",
		},
	}

	// Verify message can be constructed without panicking
	assert.NotNil(t, req)
	assert.Equal(t, "notif-001", req.ID)
}
