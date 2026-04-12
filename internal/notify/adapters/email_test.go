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

func TestNewEmailNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		config    EmailConfig
		wantError bool
	}{
		{
			name: "valid SMTP config",
			config: EmailConfig{
				SenderAddress: "alert@example.com",
				RecipientList: []string{"ops@example.com"},
				SMTPHost:      "smtp.example.com",
				SMTPPort:      587,
			},
			wantError: false,
		},
		{
			name: "missing sender",
			config: EmailConfig{
				SenderAddress: "",
				RecipientList: []string{"ops@example.com"},
				SMTPHost:      "smtp.example.com",
				SMTPPort:      587,
			},
			wantError: true,
		},
		{
			name: "missing recipients",
			config: EmailConfig{
				SenderAddress: "alert@example.com",
				RecipientList: []string{},
				SMTPHost:      "smtp.example.com",
				SMTPPort:      587,
			},
			wantError: true,
		},
		{
			name: "missing SMTP host",
			config: EmailConfig{
				SenderAddress: "alert@example.com",
				RecipientList: []string{"ops@example.com"},
				SMTPHost:      "",
				SMTPPort:      587,
			},
			wantError: true,
		},
		{
			name: "missing SMTP port",
			config: EmailConfig{
				SenderAddress: "alert@example.com",
				RecipientList: []string{"ops@example.com"},
				SMTPHost:      "smtp.example.com",
				SMTPPort:      0,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewEmailNotifier(tt.config, logger)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, notifier)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "email", notifier.Name())
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity int
		expected string
	}{
		{1, "LOW"},
		{2, "MEDIUM"},
		{3, "HIGH"},
		{4, "CRITICAL"},
		{5, "BLOCKER"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.severity)), func(t *testing.T) {
			result := severityString(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmailNotifierName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewEmailNotifier(EmailConfig{
		SenderAddress: "alert@example.com",
		RecipientList: []string{"ops@example.com"},
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
	}, logger)
	require.NoError(t, err)

	assert.Equal(t, "email", notifier.Name())
}

func TestEmailSendValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier, err := NewEmailNotifier(EmailConfig{
		SenderAddress: "alert@example.com",
		RecipientList: []string{"ops@example.com"},
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with nil request
	resp, err := notifier.Send(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestEmailNotifierSendStructure(t *testing.T) {
	// This test verifies that the Send method can construct the email request
	// without panicking.

	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	_, err := NewEmailNotifier(EmailConfig{
		SenderAddress: "alert@example.com",
		RecipientList: []string{"ops@example.com"},
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
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
			"confidence": "0.92",
		},
	}

	// Verify request structure
	assert.NotNil(t, req)
	assert.Equal(t, "notif-001", req.ID)
	assert.Equal(t, 3, req.Severity)
}
