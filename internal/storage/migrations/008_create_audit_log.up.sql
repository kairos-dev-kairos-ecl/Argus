-- 008_create_audit_log.up.sql
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,    -- 'login', 'logout', 'token_refresh', 'user_create', etc.
    resource    TEXT NOT NULL,    -- 'user', 'session', 'rule', 'config', etc.
    detail      JSONB,
    ip_address  INET,
    user_agent  TEXT,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp DESC);
CREATE INDEX idx_audit_log_action ON audit_log(action);
