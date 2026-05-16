package baseline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildSessionProfile_Modal verifies that buildSessionProfile picks the modal
// layer sequence and computes aggregates correctly.
func TestBuildSessionProfile_Modal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ttl := 30 * time.Minute

	// 3 sessions: 2 with [3,5,7], 1 with [3,7]
	// Modal sequence is [3,5,7] (appears twice)
	sessions := []sessionAgg{
		{layers: []int32{3, 5, 7}, durMS: 100.0, anomalies: 1, total: 10},
		{layers: []int32{3, 5, 7}, durMS: 200.0, anomalies: 0, total: 8},
		{layers: []int32{3, 7}, durMS: 150.0, anomalies: 2, total: 5},
	}

	prof := buildSessionProfile("app-abc", sessions, now, ttl)

	require.NotNil(t, prof)
	assert.Equal(t, "app-abc", prof.AppID)
	assert.Equal(t, int32(3), prof.SampleCount, "should have 3 sessions")
	assert.Equal(t, []int32{3, 5, 7}, prof.LayerActivationSequence, "modal sequence should be [3,5,7]")

	// AnomalyRate = (1+0+2) / (10+8+5) = 3/23
	expectedRate := 3.0 / 23.0
	assert.InDelta(t, expectedRate, prof.AnomalyRate, 1e-9, "anomaly rate should be 3/23")

	// Durations sorted: [100, 150, 200]
	// P50 idx = int(2 * 0.50) = 1 → 150ms
	// P95 idx = int(2 * 0.95) = 1 → 150ms
	assert.InDelta(t, 150.0, prof.SessionDurP50MS, 1e-9, "p50 duration")
	assert.InDelta(t, 150.0, prof.SessionDurP95MS, 1e-9, "p95 duration")

	// ComputedAt and ExpiresAt
	assert.Equal(t, now, prof.ComputedAt)
	assert.Equal(t, now.Add(ttl), prof.ExpiresAt)

	// LayerDwellMS should be initialised (not nil)
	assert.NotNil(t, prof.LayerDwellMS)
}

// TestBuildSessionProfile_Empty verifies the zero-session edge case.
func TestBuildSessionProfile_Empty(t *testing.T) {
	now := time.Now().UTC()
	ttl := 30 * time.Minute

	prof := buildSessionProfile("app-xyz", []sessionAgg{}, now, ttl)
	require.NotNil(t, prof)
	assert.Equal(t, "app-xyz", prof.AppID)
	assert.Equal(t, int32(0), prof.SampleCount)
	assert.Equal(t, now, prof.ComputedAt)
	assert.Equal(t, now.Add(ttl), prof.ExpiresAt)
}

// TestDefaultSessionBaselineConfig verifies the default configuration values.
func TestDefaultSessionBaselineConfig(t *testing.T) {
	cfg := DefaultSessionBaselineConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 10*time.Minute, cfg.ComputeInterval)
	assert.Equal(t, 24*time.Hour, cfg.HistoryWindow)
	assert.Equal(t, 5, cfg.MinSessionSize)
	assert.Equal(t, 30*time.Minute, cfg.ProfileTTL)
}
