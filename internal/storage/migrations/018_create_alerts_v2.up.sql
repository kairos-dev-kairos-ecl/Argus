-- 018_create_alerts_v2.up.sql
-- Authoritative alerts table for Phase 4 Detection Engine.
-- Supersedes migrations 005 and 010.

DROP TABLE IF EXISTS alerts CASCADE;

CREATE TABLE alerts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id          TEXT NOT NULL,
    app_id           TEXT NOT NULL DEFAULT '',
    trace_id         TEXT NOT NULL DEFAULT '',
    signal_ids       TEXT[] NOT NULL DEFAULT '{}',
    signal_count     INT NOT NULL DEFAULT 1,
    fingerprint      TEXT NOT NULL,
    severity         INT NOT NULL CHECK (severity BETWEEN 1 AND 5),
    layer            INT NOT NULL DEFAULT 0,
    category         TEXT NOT NULL DEFAULT '',
    title            TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'open'
                     CHECK (status IN ('open', 'acknowledged', 'resolved', 'suppressed')),
    context          JSONB,
    kairos_decision  JSONB,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at  TIMESTAMPTZ,
    acknowledged_by  TEXT,
    resolved_at      TIMESTAMPTZ,
    incident_id      UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_app_id ON alerts(app_id);
CREATE INDEX idx_alerts_rule_id ON alerts(rule_id);
CREATE INDEX idx_alerts_status_severity ON alerts(status, severity DESC);
CREATE INDEX idx_alerts_trace_id ON alerts(trace_id) WHERE trace_id != '';
CREATE INDEX idx_alerts_first_seen_at ON alerts(first_seen_at DESC);
