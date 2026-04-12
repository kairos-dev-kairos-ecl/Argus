-- Migration 014: Create adapter_credentials table for encrypted credential storage
-- Part of Phase 5 Wave 1: Alert Routing & Response schema foundation

CREATE TABLE IF NOT EXISTS adapter_credentials (
    credential_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adapter_type VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    credential_data BYTEA NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Index for listing all credentials of one adapter type
CREATE INDEX IF NOT EXISTS idx_adapter_credentials_type ON adapter_credentials(adapter_type);
