package notify

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NotificationRequest represents a notification to be sent by an adapter.
// It contains the payload to send and metadata needed for routing.
type NotificationRequest struct {
	ID        string            // Unique ID for tracking this notification
	AlertID   uuid.UUID         // Alert ID that triggered this notification
	Severity  int               // Severity level 1-5
	RuleID    uuid.UUID         // Rule ID that triggered this notification
	Title     string            // Title/subject of notification
	Message   string            // Message body
	Metadata  map[string]string // Additional metadata (e.g., app_id, layer)
}

// NotificationResponse represents the response from sending a notification.
type NotificationResponse struct {
	Status    string // "sent", "failed", "retrying"
	MessageID string // Identifier from the adapter (e.g., Slack TS, PagerDuty incident ID)
	Error     string // Error message if failed
	Timestamp int64  // Unix timestamp of send attempt
}

// Notifier interface defines the contract for notification adapters.
// Each adapter (Slack, PagerDuty, Email, etc.) implements this interface.
type Notifier interface {
	// Name returns the unique name of this notifier (e.g., "slack", "pagerduty")
	Name() string

	// Send sends a notification via this adapter.
	// ctx is the request context.
	// req is the notification request with alert details and message.
	// Returns a NotificationResponse with status and any error details.
	Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error)

	// Health checks if this notifier is operational.
	// Should verify connectivity to the external service if applicable.
	Health(ctx context.Context) error
}

// AdapterRegistry manages all registered notifiers.
// It provides thread-safe registration, lookup, and listing of adapters.
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]Notifier
	logger   *zap.Logger
}

// NewAdapterRegistry creates a new adapter registry.
func NewAdapterRegistry(logger *zap.Logger) *AdapterRegistry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AdapterRegistry{
		adapters: make(map[string]Notifier),
		logger:   logger,
	}
}

// Register adds a new notifier to the registry.
// It returns an error if a notifier with the same name is already registered.
func (r *AdapterRegistry) Register(notifier Notifier) error {
	if notifier == nil {
		return fmt.Errorf("notifier cannot be nil")
	}

	name := notifier.Name()
	if name == "" {
		return fmt.Errorf("notifier name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter already registered: %s", name)
	}

	r.adapters[name] = notifier
	r.logger.Info("adapter registered", zap.String("adapter", name))
	return nil
}

// Unregister removes a notifier from the registry.
func (r *AdapterRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[name]; exists {
		delete(r.adapters, name)
		r.logger.Info("adapter unregistered", zap.String("adapter", name))
	}
}

// Get retrieves a notifier by name.
// Returns nil and false if the notifier is not registered.
func (r *AdapterRegistry) Get(name string) (Notifier, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	notifier, exists := r.adapters[name]
	return notifier, exists
}

// GetAll returns a list of all registered notifiers.
func (r *AdapterRegistry) GetAll() []Notifier {
	r.mu.RLock()
	defer r.mu.RUnlock()

	notifiers := make([]Notifier, 0, len(r.adapters))
	for _, notifier := range r.adapters {
		notifiers = append(notifiers, notifier)
	}
	return notifiers
}

// List returns the names of all registered notifiers.
func (r *AdapterRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered adapters.
func (r *AdapterRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.adapters)
}

// HealthCheck verifies all registered adapters are operational.
// Returns a map of adapter name -> health check error (nil if healthy).
func (r *AdapterRegistry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	adapters := make(map[string]Notifier)
	for name, notifier := range r.adapters {
		adapters[name] = notifier
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	for name, notifier := range adapters {
		results[name] = notifier.Health(ctx)
	}
	return results
}
