-- 008_create_audit_log.up.sql
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID,
    action      TEXT NOT NULL,    -- 'create_rule', 'delete_api_key', 'update_config', etc.
    resource    TEXT NOT NULL,
    resource_id TEXT,
    details     JSONB,
    ip_address  INET,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_log_actor_id ON audit_log(actor_id);
CREATE INDEX idx_audit_log_occurred_at ON audit_log(occurred_at DESC);
