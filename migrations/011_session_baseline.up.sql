CREATE TABLE IF NOT EXISTS session_baseline_profiles (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id           TEXT NOT NULL,
    layer_sequence   INTEGER[] NOT NULL DEFAULT '{}',
    session_dur_p50  DOUBLE PRECISION NOT NULL DEFAULT 0,
    session_dur_p95  DOUBLE PRECISION NOT NULL DEFAULT 0,
    layer_dwell_ms   JSONB NOT NULL DEFAULT '{}',
    anomaly_rate     DOUBLE PRECISION NOT NULL DEFAULT 0,
    sample_count     INTEGER NOT NULL DEFAULT 0,
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sbp_app_id ON session_baseline_profiles(app_id);
CREATE INDEX IF NOT EXISTS idx_sbp_computed ON session_baseline_profiles(computed_at DESC);
