-- 007_create_baseline_profiles.up.sql
CREATE TABLE IF NOT EXISTS baseline_profiles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        TEXT NOT NULL,
    layer         INT NOT NULL,
    category      TEXT NOT NULL,
    sample_count  INT NOT NULL DEFAULT 0,
    mean          FLOAT,
    stddev        FLOAT,
    p50           FLOAT,
    p95           FLOAT,
    p99           FLOAT,
    histogram     JSONB,
    computed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT baseline_profiles_unique UNIQUE (app_id, layer, category)
);
