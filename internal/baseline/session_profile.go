package baseline

import "time"

// SessionProfile holds a per-app session-level baseline.
// It captures the modal layer activation sequence across sessions,
// duration percentiles, and anomaly rate.
// Mirrors the session_baseline_profiles PostgreSQL table.
type SessionProfile struct {
	AppID                   string             `json:"app_id"`
	LayerActivationSequence []int32            `json:"layer_activation_sequence"`
	SessionDurP50MS         float64            `json:"session_dur_p50_ms"`
	SessionDurP95MS         float64            `json:"session_dur_p95_ms"`
	LayerDwellMS            map[string]float64 `json:"layer_dwell_ms"`
	AnomalyRate             float64            `json:"anomaly_rate"`
	SampleCount             int32              `json:"sample_count"`
	ComputedAt              time.Time          `json:"computed_at"`
	ExpiresAt               time.Time          `json:"expires_at"`
}

// SessionProfileKey returns the Redis key for a session baseline profile.
func SessionProfileKey(appID string) string { return "session_baseline:" + appID }

// SessionProfileRedisTTL is the TTL for session profiles in Redis.
const SessionProfileRedisTTL = 30 * time.Minute
