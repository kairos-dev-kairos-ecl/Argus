package notify

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAlertDispatcher_Dispatch(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	// Create a mock adapter
	called := atomic.Bool{}
	notifier := &MockNotifier{
		name: "test",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			called.Store(true)
			return &NotificationResponse{Status: "sent"}, nil
		},
	}
	registry.Register(notifier)

	config := &DispatcherConfig{
		WorkerCount:   2,
		QueueCapacity: 100,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	alertID := uuid.New()
	ruleID := uuid.New()
	job := &DispatchJob{
		AlertID: alertID,
		Targets: []string{"test"},
		Notification: &NotificationRequest{
			ID:      "notif-1",
			AlertID: alertID,
			RuleID:  ruleID,
			Title:   "Test Alert",
			Message: "Test message",
		},
	}

	err = dispatcher.Dispatch(job)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	assert.True(t, called.Load())
	stats := dispatcher.Stats()
	assert.Equal(t, uint64(1), stats["accepted"])
	assert.Equal(t, uint64(1), stats["successful"])
}

func TestAlertDispatcher_QueueCapacity(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	config := &DispatcherConfig{
		WorkerCount:   1,
		QueueCapacity: 10,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	// Verify capacity is set correctly
	stats := dispatcher.Stats()
	assert.Equal(t, uint64(10), stats["queue_cap"])
	assert.Equal(t, uint64(0), stats["queue_len"])
}

func TestAlertDispatcher_MultipleAdapters(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	// Create multiple adapters
	slackCalled := atomic.Bool{}
	pdCalled := atomic.Bool{}

	slackNotifier := &MockNotifier{
		name: "slack",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			slackCalled.Store(true)
			return &NotificationResponse{Status: "sent", MessageID: "slack-123"}, nil
		},
	}
	pdNotifier := &MockNotifier{
		name: "pagerduty",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			pdCalled.Store(true)
			return &NotificationResponse{Status: "sent", MessageID: "pd-456"}, nil
		},
	}

	registry.Register(slackNotifier)
	registry.Register(pdNotifier)

	config := &DispatcherConfig{
		WorkerCount:   2,
		QueueCapacity: 100,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	job := &DispatchJob{
		AlertID: uuid.New(),
		Targets: []string{"slack", "pagerduty"},
		Notification: &NotificationRequest{
			ID:      "notif-1",
			Title:   "Test Alert",
			Message: "Test message",
		},
	}

	err = dispatcher.Dispatch(job)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	assert.True(t, slackCalled.Load())
	assert.True(t, pdCalled.Load())
}

func TestAlertDispatcher_FailureHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	// Create a notifier that always fails
	failingNotifier := &MockNotifier{
		name: "failing",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			return nil, errors.New("send failed")
		},
	}
	registry.Register(failingNotifier)

	config := &DispatcherConfig{
		WorkerCount:   1,
		QueueCapacity: 100,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	job := &DispatchJob{
		AlertID: uuid.New(),
		Targets: []string{"failing"},
		Notification: &NotificationRequest{
			ID:      "notif-1",
			Title:   "Test Alert",
			Message: "Test message",
		},
	}

	err = dispatcher.Dispatch(job)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	stats := dispatcher.Stats()
	assert.Equal(t, uint64(1), stats["failed"])
}

func TestAlertDispatcher_GracefulShutdown(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{
		name: "slow",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			time.Sleep(100 * time.Millisecond)
			return &NotificationResponse{Status: "sent"}, nil
		},
	}
	registry.Register(notifier)

	config := &DispatcherConfig{
		WorkerCount:             1,
		QueueCapacity:           100,
		SendTimeout:             5 * time.Second,
		GracefulShutdownTimeout: 5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)

	// Dispatch multiple jobs
	for i := 0; i < 5; i++ {
		job := &DispatchJob{
			AlertID: uuid.New(),
			Targets: []string{"slow"},
			Notification: &NotificationRequest{
				ID:      "notif-" + string(rune(i)),
				Title:   "Alert",
				Message: "Message",
			},
		}
		dispatcher.Dispatch(job)
	}

	// Shutdown should drain the queue
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = dispatcher.Shutdown(ctx)
	assert.NoError(t, err)

	stats := dispatcher.Stats()
	assert.Equal(t, uint64(5), stats["accepted"])
}

func TestAlertDispatcher_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	config := &DispatcherConfig{
		WorkerCount:   1,
		QueueCapacity: 100,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	stats := dispatcher.Stats()
	assert.Equal(t, uint64(0), stats["accepted"])
	assert.Equal(t, uint64(0), stats["successful"])
	assert.Equal(t, uint64(0), stats["failed"])
	assert.Equal(t, uint64(0), stats["dropped"])
	assert.Equal(t, uint64(100), stats["queue_cap"])
}

func TestAlertDispatcher_ConcurrentDispatches(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewAdapterRegistry(logger)

	notifier := &MockNotifier{
		name: "test",
		sendFn: func(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
			return &NotificationResponse{Status: "sent"}, nil
		},
	}
	registry.Register(notifier)

	config := &DispatcherConfig{
		WorkerCount:   4,
		QueueCapacity: 1000,
		SendTimeout:   5 * time.Second,
	}
	dispatcher, err := NewAlertDispatcher(config, registry, logger)
	require.NoError(t, err)
	defer dispatcher.Shutdown(context.Background())

	var wg sync.WaitGroup
	numJobs := 100

	wg.Add(numJobs)
	for i := 0; i < numJobs; i++ {
		go func(idx int) {
			defer wg.Done()
			job := &DispatchJob{
				AlertID: uuid.New(),
				Targets: []string{"test"},
				Notification: &NotificationRequest{
					ID:      "notif-" + string(rune(idx)),
					Title:   "Alert",
					Message: "Message",
				},
			}
			dispatcher.Dispatch(job)
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	stats := dispatcher.Stats()
	assert.Equal(t, uint64(numJobs), stats["accepted"])
	assert.Equal(t, uint64(numJobs), stats["successful"])
}
