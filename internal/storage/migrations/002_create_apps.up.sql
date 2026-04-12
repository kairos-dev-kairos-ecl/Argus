-- 002_create_apps.up.sql
CREATE TABLE IF NOT EXISTS apps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id      TEXT NOT NULL UNIQUE,   -- matches ArgusSignal.source.app_id
    name        TEXT NOT NULL,
    description TEXT,
    owner_id    UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    config      JSONB NOT NULL DEFAULT '{}'
);
