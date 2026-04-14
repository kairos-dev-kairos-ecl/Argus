-- 003_create_users.up.sql
CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    role                TEXT NOT NULL DEFAULT 'viewer',  -- 'admin', 'analyst', 'viewer'
    password_hash       TEXT,
    status              TEXT NOT NULL DEFAULT 'active',  -- 'active', 'inactive', 'locked'
    password_changed_at TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    last_login_ip       TEXT,
    failed_login_count  INTEGER NOT NULL DEFAULT 0,
    locked_until        TIMESTAMPTZ,
    created_by          UUID,
    invited_at          TIMESTAMPTZ,
    mfa_secret          TEXT,
    external_provider   TEXT,
    external_id         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
