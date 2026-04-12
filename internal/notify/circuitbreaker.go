package notify

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState string

const (
	StateClosed CircuitBreakerState = "closed"   // Normal operation, requests pass through
	StateOpen   CircuitBreakerState = "open"     // Failing, requests rejected immediately
	StateHalf   CircuitBreakerState = "half-open" // Testing if service recovered
)

// CircuitBreakerConfig holds configuration for CircuitBreaker.
type CircuitBreakerConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 3)
	MaxRetries int

	// InitialBackoff is the initial backoff duration for the first retry (default: 1s)
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration (default: 32s)
	MaxBackoff time.Duration

	// BackoffMultiplier is the exponential backoff multiplier (default: 2)
	BackoffMultiplier float64

	// OpenThreshold is the failure rate threshold to open the circuit (default: 50%)
	OpenThreshold float64

	// HalfOpenMaxAttempts is the number of requests to try in half-open state (default: 1)
	HalfOpenMaxAttempts int

	// HalfOpenTimeout is the duration to stay in half-open before reopening (default: 30s)
	HalfOpenTimeout time.Duration
}

// DefaultCircuitBreakerConfig returns a sensible default configuration.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxRetries:          3,
		InitialBackoff:      1 * time.Second,
		MaxBackoff:          32 * time.Second,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     30 * time.Second,
	}
}

// CircuitBreaker implements a circuit breaker pattern with exponential backoff.
// It protects against cascading failures by failing fast when a service is unhealthy.
type CircuitBreaker struct {
	config    *CircuitBreakerConfig
	mu        sync.RWMutex
	state     CircuitBreakerState
	lastError error

	// Metrics
	totalRequests   atomic.Uint64
	totalFailures   atomic.Uint64
	totalSuccesses  atomic.Uint64
	consecutiveFails atomic.Uint32

	// State transition tracking
	openedAt      time.Time
	halfOpenCount atomic.Uint32
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen returns true if the circuit is currently open (failing fast).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateOpen
}

// IsClosed returns true if the circuit is closed (normal operation).
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateClosed
}

// IsHalfOpen returns true if the circuit is half-open (testing recovery).
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateHalf
}

// Execute wraps a function with circuit breaker logic and retry with exponential backoff.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	state := cb.State()

	// If circuit is open, fail fast
	if state == StateOpen {
		cb.mu.RLock()
		openedAt := cb.openedAt
		cb.mu.RUnlock()

		// Check if we should transition to half-open
		if time.Since(openedAt) > cb.config.HalfOpenTimeout {
			cb.mu.Lock()
			if cb.state == StateOpen && time.Since(openedAt) > cb.config.HalfOpenTimeout {
				cb.state = StateHalf
				cb.halfOpenCount.Store(0)
				cb.mu.Unlock()
			} else {
				cb.mu.Unlock()
				return fmt.Errorf("circuit breaker is open")
			}
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	// Handle half-open state: limit concurrent attempts
	if cb.IsHalfOpen() {
		currentCount := cb.halfOpenCount.Load()
		if currentCount >= uint32(cb.config.HalfOpenMaxAttempts) {
			return fmt.Errorf("circuit breaker is half-open and max attempts exceeded")
		}
		cb.halfOpenCount.Add(1)
	}

	// Execute with retries and exponential backoff
	var lastErr error
	backoff := cb.config.InitialBackoff

	for attempt := 0; attempt <= cb.config.MaxRetries; attempt++ {
		cb.totalRequests.Add(1)

		err := fn(ctx)
		if err == nil {
			// Success: reset failure counters and close circuit if half-open
			cb.totalSuccesses.Add(1)
			cb.consecutiveFails.Store(0)

			cb.mu.Lock()
			if cb.state == StateHalf {
				cb.state = StateClosed
			}
			cb.mu.Unlock()

			return nil
		}

		// Failure: record it
		lastErr = err
		cb.totalFailures.Add(1)
		cb.consecutiveFails.Add(1)

		// Check if we should open the circuit
		if cb.shouldOpen() {
			cb.mu.Lock()
			cb.state = StateOpen
			cb.openedAt = time.Now()
			cb.lastError = lastErr
			cb.mu.Unlock()
			return fmt.Errorf("circuit breaker opened after %d failures: %w", attempt+1, lastErr)
		}

		// If not the last attempt, wait before retrying
		if attempt < cb.config.MaxRetries {
			select {
			case <-time.After(backoff):
				// Continue to next retry
			case <-ctx.Done():
				return ctx.Err()
			}

			// Calculate next backoff with exponential growth
			nextBackoff := time.Duration(float64(backoff) * cb.config.BackoffMultiplier)
			if nextBackoff > cb.config.MaxBackoff {
				nextBackoff = cb.config.MaxBackoff
			}
			backoff = nextBackoff
		}
	}

	// All retries exhausted
	cb.mu.Lock()
	if cb.state == StateHalf {
		// Half-open failed, reopen
		cb.state = StateOpen
		cb.openedAt = time.Now()
	}
	cb.lastError = lastErr
	cb.mu.Unlock()

	return fmt.Errorf("all %d retries failed: %w", cb.config.MaxRetries+1, lastErr)
}

// shouldOpen determines if the circuit should be opened based on failure rate.
// Only opens if we have at least 5 requests (to avoid false positives early on)
func (cb *CircuitBreaker) shouldOpen() bool {
	total := cb.totalRequests.Load()
	// Don't open circuit until we have enough data points
	if total < 5 {
		return false
	}

	failures := cb.totalFailures.Load()
	failureRate := float64(failures) / float64(total)

	return failureRate >= cb.config.OpenThreshold
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.lastError = nil
	cb.totalRequests.Store(0)
	cb.totalFailures.Store(0)
	cb.totalSuccesses.Store(0)
	cb.consecutiveFails.Store(0)
	cb.halfOpenCount.Store(0)
}

// LastError returns the last recorded error.
func (cb *CircuitBreaker) LastError() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastError
}

// Stats returns current circuit breaker statistics.
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	return map[string]interface{}{
		"state":              cb.State(),
		"total_requests":     cb.totalRequests.Load(),
		"total_successes":    cb.totalSuccesses.Load(),
		"total_failures":     cb.totalFailures.Load(),
		"consecutive_fails":  cb.consecutiveFails.Load(),
		"success_rate":       cb.getSuccessRate(),
		"failure_rate":       cb.getFailureRate(),
	}
}

// getSuccessRate returns the success rate as a percentage.
func (cb *CircuitBreaker) getSuccessRate() float64 {
	total := cb.totalRequests.Load()
	if total == 0 {
		return 100.0
	}
	successes := cb.totalSuccesses.Load()
	return (float64(successes) / float64(total)) * 100.0
}

// getFailureRate returns the failure rate as a percentage.
func (cb *CircuitBreaker) getFailureRate() float64 {
	total := cb.totalRequests.Load()
	if total == 0 {
		return 0.0
	}
	failures := cb.totalFailures.Load()
	return (float64(failures) / float64(total)) * 100.0
}
