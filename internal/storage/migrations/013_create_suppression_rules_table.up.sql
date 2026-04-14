-- Migration 013: Create suppression_rules table for time-based and pattern-based suppression
-- Part of Phase 5 Wave 1: Alert Routing & Response schema foundation

CREATE TABLE IF NOT EXISTS suppression_rules (
    suppression_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES detection_rules(id) ON DELETE CASCADE,
    starts_at TIMESTAMP NOT NULL,
    ends_at TIMESTAMP NOT NULL,
    pattern_type VARCHAR(32) NULL,
    pattern_config JSONB NULL,
    reason VARCHAR(500) NULL,
    created_by VARCHAR(255) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for suppression window evaluation
CREATE INDEX IF NOT EXISTS idx_suppression_rules_rule_id ON suppression_rules(rule_id);
CREATE INDEX IF NOT EXISTS idx_suppression_rules_time_window ON suppression_rules(starts_at, ends_at);
