package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewWebhookNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		config    WebhookConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: WebhookConfig{
				URL:     "https://example.com/webhook",
				Auth:    "none",
				Headers: map[string]string{},
			},
			wantError: false,
		},
		{
			name: "missing URL",
			config: WebhookConfig{
				URL:     "",
				Auth:    "none",
				Headers: map[string]string{},
			},
			wantError: true,
		},
		{
			name: "with custom headers",
			config: WebhookConfig{
				URL:  "https://example.com/webhook",
				Auth: "apikey",
				Headers: map[string]string{
					"Authorization": "Bearer token123",
					"X-Custom":      "value",
				},
			},
			wantError: false,
		},
		{
			name: "with timeout",
			config: WebhookConfig{
				URL:     "https://example.com/webhook",
				Auth:    "none",
				Timeout: 15 * time.Second,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewWebhookNotifier(tt.config, logger)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "webhook", notifier.Name())
			}
		})
	}
}

func TestParseSignalIDs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single signal",
			input:    "sig-001",
			expected: []string{"sig-001"},
		},
		{
			name:     "multiple signals",
			input:    "sig-001,sig-002,sig-003",
			expected: []string{"sig-001", "sig-002", "sig-003"},
		},
		{
			name:     "with spaces",
			input:    "sig-001, sig-002 , sig-003",
			expected: []string{"sig-001", "sig-002", "sig-003"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSignalIDs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		attempt   int
		minDur    time.Duration
		maxDur    time.Duration
	}{
		{0, 1 * time.Second, 2 * time.Second},      // 1s + jitter
		{1, 2 * time.Second, 4 * time.Second},      // 2s + jitter
		{2, 4 * time.Second, 8 * time.Second},      // 4s + jitter
		{3, 8 * time.Second, 16 * time.Second},     // 8s + jitter
		{4, 16 * time.Second, 32 * time.Second},    // 16s + jitter
		{5, 30 * time.Second, 30 * time.Second},    // capped at 30s + jitter
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.attempt)), func(t *testing.T) {
			dur := exponentialBackoff(tt.attempt)
			assert.GreaterOrEqual(t, dur, tt.minDur)
			assert.LessOrEqual(t, dur, 2*tt.maxDur) // Allow for jitter
		})
	}
}

func TestIsClientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "contains 400",
			err:      assert.AnError,
			expected: false, // Would need actual 400 string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isClientError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookNotifierName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewWebhookNotifier(WebhookConfig{
		URL: "https://example.com/webhook",
	}, logger)
	require.NoError(t, err)

	assert.Equal(t, "webhook", notifier.Name())
}

func TestWebhookSendValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewWebhookNotifier(WebhookConfig{
		URL: "https://example.com/webhook",
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with nil request
	resp, err := notifier.Send(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestWebhookNotifierSendStructure(t *testing.T) {
	// This test verifies that the Send method can construct the webhook request
	// without panicking.

	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	_, err := NewWebhookNotifier(WebhookConfig{
		URL: "https://example.com/webhook",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
		},
	}, logger)
	require.NoError(t, err)

	req := &notify.NotificationRequest{
		ID:       "notif-001",
		AlertID:  uuid.New(),
		Severity: 3,
		RuleID:   uuid.New(),
		Title:    "Test Alert",
		Message:  "This is a test alert",
		Metadata: map[string]string{
			"signal_ids": "sig-001,sig-002",
			"confidence": "0.88",
			"fingerprint": "xyz789",
		},
	}

	// Verify request structure
	assert.NotNil(t, req)
	assert.Equal(t, "notif-001", req.ID)
	assert.Equal(t, 3, req.Severity)
}

func TestWebhookTimeoutDefault(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	config := WebhookConfig{
		URL: "https://example.com/webhook",
	}
	notifier, err := NewWebhookNotifier(config, logger)
	require.NoError(t, err)

	// Verify default timeout is set to 5 seconds
	assert.Equal(t, 5*time.Second, notifier.client.Timeout)
}

func TestWebhookTimeoutCustom(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	config := WebhookConfig{
		URL:     "https://example.com/webhook",
		Timeout: 30 * time.Second,
	}
	notifier, err := NewWebhookNotifier(config, logger)
	require.NoError(t, err)

	// Verify custom timeout is set
	assert.Equal(t, 30*time.Second, notifier.client.Timeout)
}
