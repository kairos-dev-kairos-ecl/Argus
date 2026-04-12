-- Migration 009: Create baseline_profiles table for persistent baseline storage
-- This extends Phase 2 schema with baseline profile persistence for Phase 3

CREATE TABLE IF NOT EXISTS baseline_profiles (
    app_id VARCHAR(255) NOT NULL,
    layer VARCHAR(50) NOT NULL,
    category VARCHAR(255) NOT NULL,
    count BIGINT,
    mean DOUBLE PRECISION,
    stddev DOUBLE PRECISION,
    min DOUBLE PRECISION,
    max DOUBLE PRECISION,
    computed_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (app_id, layer, category)
);

-- Index for efficient lookups by updated_at (useful for cleanup/refresh)
CREATE INDEX IF NOT EXISTS idx_baseline_profiles_updated_at ON baseline_profiles(updated_at DESC);

-- Index for querying by app_id
CREATE INDEX IF NOT EXISTS idx_baseline_profiles_app_id ON baseline_profiles(app_id);
