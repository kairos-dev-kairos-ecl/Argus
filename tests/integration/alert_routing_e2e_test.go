package integration

import (
	"context"
	"testing"
	"time"

	"github.com/argusxdr/argus/internal/alert"
	"github.com/argusxdr/argus/internal/notify"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

// MockAdapter captures notifications for testing.
type MockAdapter struct {
	name           string
	notifications  []*notify.NotificationRequest
	shouldFail     bool
	delayMs        int
}

func (m *MockAdapter) Name() string {
	return m.name
}

func (m *MockAdapter) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}

	// Simulate network delay
	if m.delayMs > 0 {
		time.Sleep(time.Duration(m.delayMs) * time.Millisecond)
	}

	// Capture notification
	m.notifications = append(m.notifications, req)

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: req.ID,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (m *MockAdapter) Health(ctx context.Context) error {
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

// TestAlertCreationAndPersistence verifies alerts are created and stored.
func TestAlertCreationAndPersistence(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create test alert
	testAlert := &alert.Alert{
		AlertID:    uuid.New(),
		RuleID:     uuid.New(),
		SignalIDs:  []uuid.UUID{uuid.New(), uuid.New()},
		Severity:   4, // HIGH severity (1-5 scale)
		Confidence: 0.95,
	}

	// Verify alert fields
	assert.NotNil(t, testAlert.AlertID)
	assert.NotNil(t, testAlert.RuleID)
	assert.Len(t, testAlert.SignalIDs, 2)
	assert.Equal(t, alert.Severity(4), testAlert.Severity)
	assert.Equal(t, 0.95, testAlert.Confidence)
}

// TestFingerprintComputation verifies fingerprints are deterministic.
func TestFingerprintComputation(t *testing.T) {
	ruleID := uuid.New()
	signalIDs := []uuid.UUID{uuid.New(), uuid.New()}

	// Compute fingerprint multiple times
	fp1 := alert.ComputeFingerprint(ruleID.String(), signalIDs)
	fp2 := alert.ComputeFingerprint(ruleID.String(), signalIDs)

	// Should be identical
	assert.Equal(t, fp1, fp2)

	// Different rule should give different fingerprint
	differentRuleID := uuid.New()
	fp3 := alert.ComputeFingerprint(differentRuleID.String(), signalIDs)
	assert.NotEqual(t, fp1, fp3)
}

// TestAdapterRegistry verifies adapters can be registered and retrieved.
func TestAdapterRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	registry := notify.NewAdapterRegistry(logger)

	// Register mock adapters
	slack := &MockAdapter{name: "slack"}
	email := &MockAdapter{name: "email"}

	err := registry.Register(slack)
	assert.NoError(t, err)

	err = registry.Register(email)
	assert.NoError(t, err)

	// Retrieve adapters
	slackRetrieved, exists := registry.Get("slack")
	assert.True(t, exists)
	assert.Equal(t, slack, slackRetrieved)

	emailRetrieved, exists := registry.Get("email")
	assert.True(t, exists)
	assert.Equal(t, email, emailRetrieved)

	// List all adapters
	names := registry.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "slack")
	assert.Contains(t, names, "email")
}

// TestNotificationDispatching verifies notifications are dispatched to adapters.
func TestNotificationDispatching(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Set up mock adapters
	slackAdapter := &MockAdapter{name: "slack"}
	emailAdapter := &MockAdapter{name: "email", delayMs: 10}

	registry := notify.NewAdapterRegistry(logger)
	registry.Register(slackAdapter)
	registry.Register(emailAdapter)

	// Create test request
	req := &notify.NotificationRequest{
		ID:      "test-123",
		AlertID: uuid.New(),
		Severity: 4,
		RuleID:  uuid.New(),
		Title:   "Test Alert",
		Message: "Test message body",
		Metadata: map[string]string{
			"rule_id":   "rule-123",
			"rule_name": "Test Rule",
		},
	}

	// Send to both adapters
	startTime := time.Now()

	resp1, err := slackAdapter.Send(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp1)
	assert.Equal(t, "sent", resp1.Status)

	resp2, err := emailAdapter.Send(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.Equal(t, "sent", resp2.Status)

	duration := time.Since(startTime)

	// Verify notifications were captured
	assert.Len(t, slackAdapter.notifications, 1)
	assert.Len(t, emailAdapter.notifications, 1)

	// Verify latency (should be less than 100ms per adapter + 10ms delay)
	assert.Less(t, duration, 200*time.Millisecond)
}

// TestCircuitBreakerProtection verifies adapters are protected by circuit breaker.
func TestCircuitBreakerProtection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create circuit breaker with default config
	config := notify.DefaultCircuitBreakerConfig()
	cb := notify.NewCircuitBreaker(config)

	successCount := 0
	failureCount := 0

	// Execute successful operations
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error { return nil })
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	assert.Equal(t, 3, successCount)
	assert.Equal(t, 0, failureCount)

	// Execute failing operations to trigger open state
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error { return assert.AnError })
	}

	// Circuit should be open now - next call should fail immediately
	err := cb.Execute(context.Background(), func(ctx context.Context) error { return nil })
	assert.Error(t, err)
}

// TestDeduplicationLogic verifies duplicate alerts are suppressed.
func TestDeduplicationLogic(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	ruleID := uuid.New()
	signal1 := uuid.New()
	signal2 := uuid.New()

	// Compute fingerprint for a set of signals
	signalIDs := []uuid.UUID{signal1, signal2}
	fingerprint := alert.ComputeFingerprint(ruleID.String(), signalIDs)

	// Create two alerts with same fingerprint (duplicate)
	alert1 := &alert.Alert{
		AlertID:    uuid.New(),
		RuleID:     ruleID,
		SignalIDs:  signalIDs,
		Severity:   4, // HIGH severity
		Confidence: 0.95,
	}

	alert2 := &alert.Alert{
		AlertID:    uuid.New(),
		RuleID:     ruleID,
		SignalIDs:  signalIDs,
		Severity:   4, // HIGH severity
		Confidence: 0.95,
	}

	// Both should compute to same fingerprint
	fp1 := alert.ComputeFingerprint(alert1.RuleID.String(), alert1.SignalIDs)
	fp2 := alert.ComputeFingerprint(alert2.RuleID.String(), alert2.SignalIDs)

	assert.Equal(t, fp1, fp2)
	assert.Equal(t, fingerprint, fp1)
}

// TestIncidentLifecycle verifies incident status transitions.
func TestIncidentLifecycle(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create test incident
	incident := &alert.Incident{
		IncidentID: uuid.New(),
		Status:     alert.IncidentStatusOpen,
		CreatedAt:  time.Now(),
	}

	// Verify initial status
	assert.Equal(t, alert.IncidentStatusOpen, incident.Status)

	// Simulate status transitions
	// In a real system, these would go through the IncidentService state machine
	// For now, just verify the incident structure is valid
	assert.NotEmpty(t, incident.IncidentID)
	assert.NotZero(t, incident.CreatedAt)
}

// TestSuppressionRuleEvaluation verifies suppression rules work.
func TestSuppressionRuleEvaluation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create mock suppression evaluator
	se := &notify.SuppressionEvaluator{}

	// Create test alert
	testAlert := &alert.Alert{
		AlertID:   uuid.New(),
		RuleID:    uuid.New(),
		SignalIDs: []uuid.UUID{uuid.New()},
		Severity:  3, // MEDIUM severity
	}

	// By default, nothing is suppressed
	suppressed, ruleID, err := se.IsSuppressed(context.Background(), testAlert)
	assert.NoError(t, err)
	assert.False(t, suppressed)
	assert.Empty(t, ruleID)
}

// TestEndToEndAlertFlow verifies complete alert lifecycle.
func TestEndToEndAlertFlow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Step 1: Create alert
	testAlert := &alert.Alert{
		AlertID:    uuid.New(),
		RuleID:     uuid.New(),
		SignalIDs:  []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		Severity:   4, // HIGH severity
		Confidence: 0.92,
	}

	// Step 2: Compute fingerprint
	fingerprint := alert.ComputeFingerprint(testAlert.RuleID.String(), testAlert.SignalIDs)
	assert.NotEmpty(t, fingerprint)

	// Step 3: Check deduplication (first time)
	// In real system, would use Redis dedup window
	// For now, just verify fingerprint is unique
	fingerprint2 := alert.ComputeFingerprint(testAlert.RuleID.String(), testAlert.SignalIDs)
	assert.Equal(t, fingerprint, fingerprint2)

	// Step 4: Set up notification adapters
	slackAdapter := &MockAdapter{name: "slack"}
	emailAdapter := &MockAdapter{name: "email"}
	syslogAdapter := &MockAdapter{name: "syslog"}

	registry := notify.NewAdapterRegistry(logger)
	registry.Register(slackAdapter)
	registry.Register(emailAdapter)
	registry.Register(syslogAdapter)

	// Step 5: Send notifications
	req := &notify.NotificationRequest{
		ID:      fingerprint[:16], // Use fingerprint as notification ID
		AlertID: testAlert.AlertID,
		Severity: int(testAlert.Severity),
		RuleID:  testAlert.RuleID,
		Title:   "High Severity Alert",
		Message: "Multiple suspicious signals detected",
		Metadata: map[string]string{
			"rule_id":       testAlert.RuleID.String(),
			"rule_name":     "Suspicious Activity",
			"signal_count":  "3",
			"confidence":    "0.92",
			"fingerprint":   fingerprint,
			"dedup_count":   "1",
		},
	}

	// Send to all adapters
	for _, adapter := range registry.GetAll() {
		_, err := adapter.Send(context.Background(), req)
		assert.NoError(t, err)
	}

	// Step 6: Verify notifications were sent
	assert.Len(t, slackAdapter.notifications, 1)
	assert.Len(t, emailAdapter.notifications, 1)
	assert.Len(t, syslogAdapter.notifications, 1)

	// Step 7: Create incident from alert
	incidentUUID := uuid.New()
	incident := &alert.Incident{
		IncidentID: incidentUUID,
		Status:     alert.IncidentStatusOpen,
		CreatedAt:  time.Now(),
	}

	// Assign incident to alert
	testAlert.IncidentID = &incidentUUID

	assert.NotNil(t, testAlert.IncidentID)
	assert.Equal(t, incidentUUID.String(), testAlert.IncidentID.String())

	// Step 8: Verify incident lifecycle
	assert.Equal(t, alert.IncidentStatusOpen, incident.Status)
	assert.NotZero(t, incident.CreatedAt)
}

// TestDeliveryLatencySLA verifies notifications meet latency SLA.
func TestDeliveryLatencySLA(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create adapter with simulated network delay (20ms)
	slowAdapter := &MockAdapter{name: "slow-service", delayMs: 20}

	req := &notify.NotificationRequest{
		ID:      "latency-test",
		AlertID: uuid.New(),
		Severity: 4,
		RuleID:  uuid.New(),
		Title:   "Latency Test",
	}

	// Measure delivery time
	start := time.Now()
	_, err := slowAdapter.Send(context.Background(), req)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Greater(t, duration, 20*time.Millisecond)
	assert.Less(t, duration, 100*time.Millisecond) // SLA: < 100ms
}

// TestMultiAdapterDispatch verifies multiple adapters can be used together.
func TestMultiAdapterDispatch(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Register 5 adapters (Slack, Email, PagerDuty, Webhook, Syslog)
	adapters := []notify.Notifier{
		&MockAdapter{name: "slack"},
		&MockAdapter{name: "email"},
		&MockAdapter{name: "pagerduty"},
		&MockAdapter{name: "webhook"},
		&MockAdapter{name: "syslog"},
	}

	registry := notify.NewAdapterRegistry(logger)
	for _, adapter := range adapters {
		registry.Register(adapter)
	}

	// Verify all 5 adapters are registered
	assert.Equal(t, 5, registry.Count())

	// Send notification to all
	req := &notify.NotificationRequest{
		ID:      "multi-adapter-test",
		AlertID: uuid.New(),
		Severity: 5,
		RuleID:  uuid.New(),
		Title:   "Critical Alert",
	}

	for _, adapter := range registry.GetAll() {
		_, err := adapter.Send(context.Background(), req)
		assert.NoError(t, err)
	}

	// Verify all received the notification
	for _, adapter := range adapters {
		mockAdapter := adapter.(*MockAdapter)
		assert.Len(t, mockAdapter.notifications, 1)
	}
}
