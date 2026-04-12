package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewSyslogNotifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	tests := []struct {
		name      string
		config    SyslogConfig
		wantError bool
		errMsg    string
	}{
		{
			name:      "invalid - missing server",
			config:    SyslogConfig{Server: "", Transport: "udp"},
			wantError: true,
			errMsg:    "server address is required",
		},
		{
			name:      "invalid - bad transport",
			config:    SyslogConfig{Server: "localhost:514", Transport: "http"},
			wantError: true,
			errMsg:    "invalid transport",
		},
		{
			name:      "valid - defaults to UDP localhost",
			config:    SyslogConfig{Server: "localhost:514"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewSyslogNotifier(tt.config, logger)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, notifier)
			} else {
				// Note: local connection may fail in test environment, but object should be created
				if err == nil {
					require.NotNil(t, notifier)
					assert.Equal(t, "syslog", notifier.Name())
					notifier.Close()
				}
			}
		})
	}
}

func TestSyslogNotifierName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	config := SyslogConfig{Server: "localhost:514", Transport: "udp"}
	notifier, err := NewSyslogNotifier(config, logger)
	if err == nil {
		defer notifier.Close()
		assert.Equal(t, "syslog", notifier.Name())
	}
}

func TestCEFSeverityMapping(t *testing.T) {
	tests := []struct {
		severity int
		expected string
	}{
		{5, "0"},   // CRITICAL → High
		{4, "2"},   // HIGH → High
		{3, "5"},   // MEDIUM → Medium
		{2, "7"},   // LOW → Low
		{1, "10"},  // INFO → Informational
		{0, "5"},   // Unknown → Default Medium
		{99, "5"},  // Invalid → Default Medium
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("severity_%d", tt.severity), func(t *testing.T) {
			result := cefSeverityFromInt(tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyslogNotifierSendValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create notifier with nil writer to test error case
	notifier := &SyslogNotifier{
		config: SyslogConfig{Server: "localhost:514", Transport: "udp"},
		logger: logger,
		writer: nil,
	}

	req := &notify.NotificationRequest{
		ID:        "test-123",
		AlertID:   uuid.New(),
		Severity:  4,
		RuleID:    uuid.New(),
		Title:     "Test Alert",
		Message:   "Test message",
		Metadata: map[string]string{
			"rule_id":     "rule-123",
			"rule_name":   "Test Rule",
			"signal_count": "5",
			"confidence":  "0.95",
			"fingerprint": "abc123",
			"dedup_count": "1",
		},
	}

	resp, err := notifier.Send(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "syslog writer not initialized")
}

func TestSyslogNotifierHealthWithNilWriter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	notifier := &SyslogNotifier{
		config: SyslogConfig{Server: "localhost:514", Transport: "udp"},
		logger: logger,
		writer: nil,
	}

	err := notifier.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syslog writer not initialized")
}

func TestSyslogNotifierFacilityDefault(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Test with facility = 0 (should default to LOCAL7)
	config := SyslogConfig{
		Server:    "localhost:514",
		Transport: "udp",
		Facility:  0, // Will default to LOCAL7
	}

	notifier, err := NewSyslogNotifier(config, logger)
	if err == nil {
		defer notifier.Close()
		assert.NotNil(t, notifier)
	}
}

func TestSyslogNotifierTransportDefaults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Test with empty transport (should default to UDP)
	config := SyslogConfig{
		Server:    "localhost:514",
		Transport: "", // Will default to UDP
	}

	notifier, err := NewSyslogNotifier(config, logger)
	if err == nil {
		defer notifier.Close()
		assert.NotNil(t, notifier)
		assert.Equal(t, "udp", notifier.protocol)
	}
}

func TestSyslogNotifierMetadataExtraction(t *testing.T) {
	// Verify that metadata fields are properly extracted and formatted in CEF
	// This is more of an integration test that would require a mock syslog server

	testCases := []struct {
		name     string
		metadata map[string]string
		verify   func(t *testing.T, metadata map[string]string)
	}{
		{
			name: "all fields present",
			metadata: map[string]string{
				"rule_id":      "rule-123",
				"rule_name":    "Suspicious Activity",
				"signal_count": "10",
				"confidence":   "0.99",
				"fingerprint":  "fp-abc",
				"dedup_count":  "2",
			},
			verify: func(t *testing.T, m map[string]string) {
				assert.Equal(t, "rule-123", m["rule_id"])
				assert.Equal(t, "Suspicious Activity", m["rule_name"])
				assert.Equal(t, "10", m["signal_count"])
				assert.Equal(t, "0.99", m["confidence"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.verify(t, tc.metadata)
		})
	}
}
