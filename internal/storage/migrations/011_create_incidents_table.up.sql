-- Migration 011: Extend incidents table with lifecycle columns
-- The base incidents table was created in 006_create_incidents.up.sql.
-- This migration adds the status-lifecycle and escalation columns introduced
-- in Phase 5 Wave 1.  All ADD COLUMN statements are idempotent.

ALTER TABLE incidents ADD COLUMN IF NOT EXISTS acknowledged_at  TIMESTAMP         NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS closed_at        TIMESTAMP         NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assigned_to      VARCHAR(255)      NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS escalated_at     TIMESTAMP         NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS escalation_target VARCHAR(255)     NULL;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS description       TEXT             NULL;

-- Indexes for incident dashboard and detail queries
CREATE INDEX IF NOT EXISTS idx_incidents_status      ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_created_at  ON incidents(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_assigned_to ON incidents(assigned_to);
