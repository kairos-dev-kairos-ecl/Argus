-- 001_create_api_keys.up.sql
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id      TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,  -- bcrypt hash of the raw key
    name        TEXT NOT NULL,
    scopes      TEXT[] NOT NULL DEFAULT '{}',  -- ["ingest", "query", "admin"]
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_by  UUID
);
CREATE INDEX idx_api_keys_app_id ON api_keys(app_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
