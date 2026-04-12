-- Migration 012: Create routing_rules table for condition-based routing configuration
-- Part of Phase 5 Wave 1: Alert Routing & Response schema foundation

CREATE TABLE IF NOT EXISTS routing_rules (
    routing_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    condition_expr VARCHAR(2000) NOT NULL,
    targets JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NULL
);

-- Index for evaluating only active rules during dispatch
CREATE INDEX IF NOT EXISTS idx_routing_rules_enabled ON routing_rules(enabled);
