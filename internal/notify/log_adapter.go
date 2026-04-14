package notify

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

var _ Notifier = (*LogAdapter)(nil)

// LogAdapter logs notifications and keeps a copy for tests/dev inspection.
type LogAdapter struct {
	logger *zap.Logger
	mu     sync.Mutex
	sent   []*NotificationRequest
}

// NewLogAdapter creates a new log notifier.
func NewLogAdapter(logger *zap.Logger) *LogAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LogAdapter{logger: logger}
}

// Name returns the adapter name.
func (a *LogAdapter) Name() string { return "log" }

// Send logs and records the notification.
func (a *LogAdapter) Send(_ context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	if req == nil {
		return &NotificationResponse{Status: "failed", Error: "nil request", Timestamp: time.Now().Unix()}, nil
	}

	a.logger.Info("notification dispatched",
		zap.String("alert_id", req.AlertID.String()),
		zap.String("title", req.Title),
		zap.Int("severity", req.Severity),
	)

	a.mu.Lock()
	a.sent = append(a.sent, req)
	a.mu.Unlock()

	return &NotificationResponse{
		Status:    "sent",
		MessageID: req.ID,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Health always succeeds for log adapter.
func (a *LogAdapter) Health(_ context.Context) error { return nil }

// Sent returns a snapshot of sent notifications.
func (a *LogAdapter) Sent() []*NotificationRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*NotificationRequest, len(a.sent))
	copy(out, a.sent)
	return out
}

// Reset clears stored notifications.
func (a *LogAdapter) Reset() {
	a.mu.Lock()
	a.sent = nil
	a.mu.Unlock()
}
