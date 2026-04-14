package notify

import "context"

var _ Notifier = (*CircuitBreakerAdapter)(nil)

// CircuitBreakerAdapter wraps a Notifier with a CircuitBreaker.
type CircuitBreakerAdapter struct {
	inner Notifier
	cb    *CircuitBreaker
}

// NewCircuitBreakerAdapter creates a new CircuitBreakerAdapter.
func NewCircuitBreakerAdapter(inner Notifier, cb *CircuitBreaker) *CircuitBreakerAdapter {
	if cb == nil {
		cb = NewCircuitBreaker(nil)
	}
	return &CircuitBreakerAdapter{
		inner: inner,
		cb:    cb,
	}
}

// Name returns the underlying notifier name.
func (a *CircuitBreakerAdapter) Name() string {
	return a.inner.Name()
}

// Send sends notifications through the circuit breaker.
func (a *CircuitBreakerAdapter) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	var resp *NotificationResponse
	err := a.cb.Execute(ctx, func(execCtx context.Context) error {
		var sendErr error
		resp, sendErr = a.inner.Send(execCtx, req)
		return sendErr
	})
	return resp, err
}

// Health delegates to the inner notifier.
func (a *CircuitBreakerAdapter) Health(ctx context.Context) error {
	return a.inner.Health(ctx)
}
