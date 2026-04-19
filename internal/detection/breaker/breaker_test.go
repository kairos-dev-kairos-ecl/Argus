package breaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBreaker_StartsClosed(t *testing.T) {
	b := New(30 * time.Second)
	assert.Equal(t, StateClosed, b.State())
	assert.False(t, b.IsOpen())
}

func TestBreaker_TripsOnHighQueueDepth(t *testing.T) {
	b := New(30 * time.Second)
	b.Evaluate(850, 1000, 0, 500)
	assert.Equal(t, StateOpen, b.State())
	assert.True(t, b.IsOpen())
}

func TestBreaker_TripsOnHighP99(t *testing.T) {
	b := New(30 * time.Second)
	b.Evaluate(0, 1000, 600, 500)
	assert.Equal(t, StateOpen, b.State())
	assert.True(t, b.IsOpen())
}

func TestBreaker_HalfOpenAfterCooldown(t *testing.T) {
	now := time.Now()
	b := New(30 * time.Second)
	b.now = func() time.Time { return now }

	b.Evaluate(850, 1000, 0, 500) // trip
	assert.Equal(t, StateOpen, b.State())

	// Advance time past cooldown
	b.now = func() time.Time { return now.Add(31 * time.Second) }
	assert.Equal(t, StateHalfOpen, b.State())
	assert.False(t, b.IsOpen()) // HALF_OPEN returns false from IsOpen
}

func TestBreaker_SucceedFromHalfOpenClosesCircuit(t *testing.T) {
	now := time.Now()
	b := New(30 * time.Second)
	b.now = func() time.Time { return now }

	b.Evaluate(850, 1000, 0, 500) // trip → OPEN
	b.now = func() time.Time { return now.Add(31 * time.Second) }
	assert.Equal(t, StateHalfOpen, b.State())

	b.Succeed()
	assert.Equal(t, StateClosed, b.State())
}

func TestBreaker_FailFromHalfOpenReopens(t *testing.T) {
	now := time.Now()
	b := New(30 * time.Second)
	b.now = func() time.Time { return now }

	b.Evaluate(850, 1000, 0, 500) // trip → OPEN
	b.now = func() time.Time { return now.Add(31 * time.Second) }
	assert.Equal(t, StateHalfOpen, b.State())

	b.Fail()
	assert.Equal(t, StateOpen, b.State())
}
