-- 005_create_alerts.up.sql
CREATE TABLE IF NOT EXISTS alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     TEXT NOT NULL,
    app_id      TEXT NOT NULL,
    severity    TEXT NOT NULL,
    confidence  FLOAT NOT NULL,
    signal_ids  TEXT[] NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open',  -- 'open', 'acknowledged', 'resolved', 'suppressed'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    incident_id UUID,
    fingerprint TEXT,                 -- For deduplication
    CONSTRAINT alerts_status_check CHECK (status IN ('open','acknowledged','resolved','suppressed','closed'))
);
CREATE INDEX idx_alerts_app_id ON alerts(app_id);
CREATE INDEX idx_alerts_rule_id ON alerts(rule_id);
CREATE INDEX idx_alerts_created_at ON alerts(created_at DESC);
CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint) WHERE fingerprint IS NOT NULL;
