package baseline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SessionProfileStore handles persistence and retrieval of session baseline profiles.
// It uses Redis as a fast cache (30-minute TTL) and PostgreSQL as authoritative storage.
// Follows the same dual-write pattern as ProfileStore.
type SessionProfileStore struct {
	redis  *redis.Client
	pg     *pgxpool.Pool
	logger *zap.Logger
}

// NewSessionProfileStore creates a new session profile store.
func NewSessionProfileStore(r *redis.Client, pg *pgxpool.Pool, log *zap.Logger) *SessionProfileStore {
	if log == nil {
		log = zap.NewNop()
	}
	return &SessionProfileStore{redis: r, pg: pg, logger: log}
}

// Save persists a session profile to Redis (best-effort) and PostgreSQL (authoritative).
// Redis failure is non-fatal and logged as a warning.
// Returns error only if PostgreSQL upsert fails.
func (s *SessionProfileStore) Save(ctx context.Context, p *SessionProfile) error {
	if p == nil {
		return fmt.Errorf("nil profile")
	}
	if p.AppID == "" {
		return fmt.Errorf("empty app_id")
	}

	// Redis best-effort
	if s.redis != nil {
		if blob, err := json.Marshal(p); err == nil {
			if rerr := s.redis.Set(ctx, SessionProfileKey(p.AppID), blob, SessionProfileRedisTTL).Err(); rerr != nil {
				s.logger.Warn("session profile redis set failed",
					zap.String("app_id", p.AppID),
					zap.Error(rerr))
			}
		}
	}

	// PostgreSQL UPSERT (authoritative)
	dwell, _ := json.Marshal(p.LayerDwellMS)
	const q = `
		INSERT INTO session_baseline_profiles
		  (app_id, layer_sequence, session_dur_p50, session_dur_p95, layer_dwell_ms, anomaly_rate, sample_count, computed_at, expires_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9)
		ON CONFLICT (app_id) DO UPDATE SET
		  layer_sequence = EXCLUDED.layer_sequence,
		  session_dur_p50 = EXCLUDED.session_dur_p50,
		  session_dur_p95 = EXCLUDED.session_dur_p95,
		  layer_dwell_ms = EXCLUDED.layer_dwell_ms,
		  anomaly_rate = EXCLUDED.anomaly_rate,
		  sample_count = EXCLUDED.sample_count,
		  computed_at = EXCLUDED.computed_at,
		  expires_at = EXCLUDED.expires_at`

	_, err := s.pg.Exec(ctx, q,
		p.AppID, p.LayerActivationSequence,
		p.SessionDurP50MS, p.SessionDurP95MS,
		string(dwell), p.AnomalyRate, p.SampleCount,
		p.ComputedAt, p.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("upsert session profile: %w", err)
	}
	return nil
}

// Get retrieves a session profile for the given appID.
// Tries Redis first; on miss queries PostgreSQL and writes through to Redis.
// Returns (nil, nil) when not found — matches ProfileStore.GetProfile behaviour.
func (s *SessionProfileStore) Get(ctx context.Context, appID string) (*SessionProfile, error) {
	// Redis fast path
	if s.redis != nil {
		if val, err := s.redis.Get(ctx, SessionProfileKey(appID)).Result(); err == nil {
			var p SessionProfile
			if jerr := json.Unmarshal([]byte(val), &p); jerr == nil {
				return &p, nil
			}
		}
	}

	// PostgreSQL fallback
	const q = `
		SELECT app_id, layer_sequence, session_dur_p50, session_dur_p95,
		       layer_dwell_ms, anomaly_rate, sample_count, computed_at, expires_at
		FROM session_baseline_profiles
		WHERE app_id = $1
		LIMIT 1`

	var p SessionProfile
	var dwell []byte
	err := s.pg.QueryRow(ctx, q, appID).Scan(
		&p.AppID, &p.LayerActivationSequence,
		&p.SessionDurP50MS, &p.SessionDurP95MS,
		&dwell, &p.AnomalyRate, &p.SampleCount,
		&p.ComputedAt, &p.ExpiresAt,
	)
	if err != nil {
		// Not found is non-fatal — matches existing ProfileStore.GetProfile
		return nil, nil
	}
	_ = json.Unmarshal(dwell, &p.LayerDwellMS)

	// Write-through cache
	if s.redis != nil {
		if blob, jerr := json.Marshal(&p); jerr == nil {
			_ = s.redis.Set(ctx, SessionProfileKey(appID), blob, SessionProfileRedisTTL).Err()
		}
	}
	return &p, nil
}
