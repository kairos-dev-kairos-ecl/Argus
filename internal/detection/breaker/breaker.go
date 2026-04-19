package breaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation — all traffic passes
	StateOpen                  // Tripped — block or sample traffic
	StateHalfOpen              // Cooldown elapsed — probe traffic allowed
)

// CircuitBreaker is a 3-state circuit breaker.
// It is safe for concurrent use.
type CircuitBreaker struct {
	mu            sync.Mutex
	state         State
	openedAt      time.Time
	halfOpenAfter time.Duration
	now           func() time.Time // injectable for testing
}

// New creates a CircuitBreaker with the given half-open cooldown.
// If halfOpenAfter <= 0, it defaults to 30 seconds.
func New(halfOpenAfter time.Duration) *CircuitBreaker {
	if halfOpenAfter <= 0 {
		halfOpenAfter = 30 * time.Second
	}
	return &CircuitBreaker{
		state:         StateClosed,
		halfOpenAfter: halfOpenAfter,
		now:           time.Now,
	}
}

// State returns the current state, transitioning OPEN→HALF_OPEN when the
// cooldown has elapsed.
func (b *CircuitBreaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen && b.now().After(b.openedAt.Add(b.halfOpenAfter)) {
		b.state = StateHalfOpen
	}
	return b.state
}

// IsOpen returns true only when in StateOpen.
// HALF_OPEN returns false so probe traffic is allowed.
func (b *CircuitBreaker) IsOpen() bool {
	return b.State() == StateOpen
}

// Evaluate trips the breaker when queueDepth > 80% of maxDepth or p99Ms > maxP99Ms.
func (b *CircuitBreaker) Evaluate(queueDepth, maxDepth int, p99Ms, maxP99Ms float64) {
	if float64(queueDepth) > 0.8*float64(maxDepth) || p99Ms > maxP99Ms {
		b.trip()
	}
}

// Succeed transitions from HALF_OPEN to CLOSED.
// No-op if not in HALF_OPEN.
func (b *CircuitBreaker) Succeed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.state = StateClosed
	}
}

// Fail transitions from HALF_OPEN back to OPEN.
// No-op if not in HALF_OPEN.
func (b *CircuitBreaker) Fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.state = StateOpen
		b.openedAt = b.now()
	}
}

func (b *CircuitBreaker) trip() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StateOpen
	b.openedAt = b.now()
}
