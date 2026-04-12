-- Migration 010: Create alerts table with fingerprint-based deduplication
-- Part of Phase 5 Wave 1: Alert Routing & Response schema foundation

CREATE TABLE IF NOT EXISTS alerts (
    alert_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES rules(rule_id) ON DELETE CASCADE,
    signal_ids UUID[] NOT NULL,
    severity INT NOT NULL CHECK (severity BETWEEN 1 AND 5),
    confidence FLOAT8 NOT NULL CHECK (confidence >= 0.0 AND confidence <= 1.0),
    fingerprint VARCHAR(64) NOT NULL,
    dedup_count INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved', 'closed')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP NULL,
    resolved_at TIMESTAMP NULL,
    closed_at TIMESTAMP NULL,
    incident_id UUID NULL REFERENCES incidents(incident_id) ON DELETE SET NULL,
    suppressed_by_window_id UUID NULL REFERENCES suppression_rules(suppression_rule_id) ON DELETE SET NULL,
    created_by VARCHAR(255) NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for Wave 2-5 performance requirements
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint_created ON alerts(fingerprint, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_status_severity ON alerts(status, severity DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_rule_id ON alerts(rule_id);
CREATE INDEX IF NOT EXISTS idx_alerts_incident_id ON alerts(incident_id);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at DESC);
