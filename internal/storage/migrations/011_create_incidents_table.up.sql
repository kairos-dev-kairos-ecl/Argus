-- Migration 011: Create incidents table with status lifecycle
-- Part of Phase 5 Wave 1: Alert Routing & Response schema foundation

CREATE TABLE IF NOT EXISTS incidents (
    incident_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(32) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved', 'closed')),
    severity INT NOT NULL CHECK (severity BETWEEN 1 AND 5),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP NULL,
    resolved_at TIMESTAMP NULL,
    closed_at TIMESTAMP NULL,
    assigned_to VARCHAR(255) NULL,
    escalated_at TIMESTAMP NULL,
    escalation_target VARCHAR(255) NULL,
    title VARCHAR(500) NULL,
    description TEXT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for incident dashboard and incident detail queries
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_created_at ON incidents(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_assigned_to ON incidents(assigned_to);
