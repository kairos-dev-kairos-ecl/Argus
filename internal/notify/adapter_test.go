package notify

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockNotifier is a test implementation of Notifier.
type MockNotifier struct {
	name     string
	sendFn   func(context.Context, *NotificationRequest) (*NotificationResponse, error)
	healthFn func(context.Context) error
}

func (m *MockNotifier) Name() string {
	return m.name
}

func (m *MockNotifier) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, req)
	}
	return &NotificationResponse{Status: "sent"}, nil
}

func (m *MockNotifier) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}

func TestAdapterRegistry_Register(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{name: "test-slack"}

	err := registry.Register(notifier)
	assert.NoError(t, err)
	assert.Equal(t, 1, registry.Count())
}

func TestAdapterRegistry_RegisterNil(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	err := registry.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestAdapterRegistry_RegisterDuplicate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier1 := &MockNotifier{name: "slack"}
	notifier2 := &MockNotifier{name: "slack"}

	err := registry.Register(notifier1)
	assert.NoError(t, err)

	err = registry.Register(notifier2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestAdapterRegistry_Get(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{name: "pagerduty"}
	registry.Register(notifier)

	got, exists := registry.Get("pagerduty")
	assert.True(t, exists)
	assert.Equal(t, notifier, got)
}

func TestAdapterRegistry_GetNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	_, exists := registry.Get("nonexistent")
	assert.False(t, exists)
}

func TestAdapterRegistry_Unregister(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{name: "email"}
	registry.Register(notifier)
	assert.Equal(t, 1, registry.Count())

	registry.Unregister("email")
	assert.Equal(t, 0, registry.Count())
}

func TestAdapterRegistry_List(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	names := []string{"slack", "pagerduty", "email"}
	for _, name := range names {
		notifier := &MockNotifier{name: name}
		registry.Register(notifier)
	}

	got := registry.List()
	assert.Len(t, got, 3)
	assert.ElementsMatch(t, got, names)
}

func TestAdapterRegistry_GetAll(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier1 := &MockNotifier{name: "slack"}
	notifier2 := &MockNotifier{name: "pagerduty"}
	registry.Register(notifier1)
	registry.Register(notifier2)

	all := registry.GetAll()
	assert.Len(t, all, 2)
}

func TestAdapterRegistry_HealthCheck(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{
		name: "healthy",
		healthFn: func(ctx context.Context) error {
			return nil
		},
	}
	registry.Register(notifier)

	results := registry.HealthCheck(context.Background())
	assert.Len(t, results, 1)
	assert.NoError(t, results["healthy"])
}

func TestNotificationRequest(t *testing.T) {
	alertID := uuid.New()
	ruleID := uuid.New()

	req := &NotificationRequest{
		ID:        "notif-123",
		AlertID:   alertID,
		Severity:  3,
		RuleID:    ruleID,
		Title:     "High Severity Alert",
		Message:   "An anomaly was detected",
		Metadata:  map[string]string{"layer": "L5"},
	}

	assert.Equal(t, "notif-123", req.ID)
	assert.Equal(t, alertID, req.AlertID)
	assert.Equal(t, 3, req.Severity)
	assert.Equal(t, "L5", req.Metadata["layer"])
}

func TestNotificationResponse(t *testing.T) {
	resp := &NotificationResponse{
		Status:    "sent",
		MessageID: "slack-msg-123",
		Error:     "",
		Timestamp: 1234567890,
	}

	assert.Equal(t, "sent", resp.Status)
	assert.Equal(t, "slack-msg-123", resp.MessageID)
	assert.Empty(t, resp.Error)
}

func TestAdapterRegistry_ConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	// Register adapters
	for i := 0; i < 10; i++ {
		notifier := &MockNotifier{name: "adapter-" + string(rune(i))}
		err := registry.Register(notifier)
		require.NoError(t, err)
	}

	// Concurrent reads
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				registry.Get("adapter-0")
				registry.List()
				registry.Count()
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	assert.Equal(t, 10, registry.Count())
}
