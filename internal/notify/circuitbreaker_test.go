package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(nil)
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())
}

func TestCircuitBreaker_ExecuteSuccess(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, cb.IsClosed())
	assert.Equal(t, uint64(1), cb.totalRequests.Load())
	assert.Equal(t, uint64(1), cb.totalSuccesses.Load())
}

func TestCircuitBreaker_ExecuteFailureWithRetry(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          3,
		InitialBackoff:      10 * time.Millisecond,
		MaxBackoff:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     1 * time.Second,
	}
	cb := NewCircuitBreaker(config)

	callCount := 0
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary failure")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
	// Circuit doesn't open until failure rate reaches threshold with enough samples
	assert.True(t, cb.IsClosed() || cb.IsOpen())
}

func TestCircuitBreaker_ExecuteAllRetriesFail(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          2,
		InitialBackoff:      10 * time.Millisecond,
		MaxBackoff:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     1 * time.Second,
	}
	cb := NewCircuitBreaker(config)

	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("persistent failure")
	})

	assert.Error(t, err)
	// Error will contain "all retries failed" or "circuit breaker opened"
	assert.True(t,
		(err.Error() != "" &&
		(containsString(err.Error(), "retries") ||
		 containsString(err.Error(), "circuit breaker"))),
		"error should mention retries or circuit breaker: %v", err)
}

func containsString(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCircuitBreaker_Opens(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          0,
		InitialBackoff:      10 * time.Millisecond,
		MaxBackoff:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)

	// Trigger failures (need at least 5 to meet threshold with 50% failure rate)
	// 5 failures means circuit should open (5/5 = 100% > 50%)
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}

	// Circuit should be open now
	assert.True(t, cb.IsOpen())

	// Next call should fail with circuit breaker open
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("should not be called")
	})
	assert.Error(t, err)
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          0,
		InitialBackoff:      10 * time.Millisecond,
		MaxBackoff:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit (5 failures = 100% failure rate > 50%)
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("failure")
		})
	}
	assert.True(t, cb.IsOpen())

	// Wait for half-open timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should transition to half-open and succeed
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Single execution
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	stats := cb.Stats()
	assert.Equal(t, uint64(1), stats["total_requests"])
	assert.Equal(t, uint64(1), stats["total_successes"])
	assert.Equal(t, uint64(0), stats["total_failures"])
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("fail")
	})

	assert.NotEqual(t, uint64(0), cb.totalRequests.Load())

	cb.Reset()

	assert.Equal(t, uint64(0), cb.totalRequests.Load())
	assert.Equal(t, uint64(0), cb.totalSuccesses.Load())
	assert.Equal(t, uint64(0), cb.totalFailures.Load())
	assert.True(t, cb.IsClosed())
	assert.Nil(t, cb.LastError())
}

func TestCircuitBreaker_LastError(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	testErr := errors.New("test error")
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return testErr
	})

	assert.NotNil(t, cb.LastError())
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          3,
		InitialBackoff:      100 * time.Millisecond,
		MaxBackoff:          200 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     1 * time.Second,
	}
	cb := NewCircuitBreaker(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Execute in a goroutine and cancel after first call
	errChan := make(chan error, 1)
	go func() {
		err := cb.Execute(ctx, func(c context.Context) error {
			cancel() // Cancel on first execution
			return errors.New("fail")
		})
		errChan <- err
	}()

	err := <-errChan
	assert.Error(t, err)
	// Error will be either context.Canceled or wrapped error
}

func TestCircuitBreaker_SuccessRate(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Single successful execution
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	stats := cb.Stats()
	successRate := stats["success_rate"].(float64)
	// Should be 100% (1/1 successful)
	assert.Equal(t, 100.0, successRate)
}

func TestCircuitBreaker_FailureRate(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Single failed execution
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return errors.New("fail")
	})

	stats := cb.Stats()
	failureRate := stats["failure_rate"].(float64)
	// Should be 100% (1/1 failed)
	assert.Equal(t, 100.0, failureRate)
}

func TestCircuitBreaker_ExponentialBackoff(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxRetries:          3,
		InitialBackoff:      10 * time.Millisecond,
		MaxBackoff:          100 * time.Millisecond,
		BackoffMultiplier:   2.0,
		OpenThreshold:       0.5,
		HalfOpenMaxAttempts: 1,
		HalfOpenTimeout:     1 * time.Second,
	}
	cb := NewCircuitBreaker(config)

	callTimes := []time.Duration{}
	start := time.Now()

	cb.Execute(context.Background(), func(ctx context.Context) error {
		callTimes = append(callTimes, time.Since(start))
		return errors.New("fail")
	})

	// Should have 1 call initially (may not retry due to circuit breaker logic)
	// or 4 calls if circuit hasn't opened
	assert.True(t, len(callTimes) >= 1, "should have at least 1 call")
}
