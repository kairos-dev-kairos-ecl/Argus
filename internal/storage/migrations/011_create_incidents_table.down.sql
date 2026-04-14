-- Rollback 011: Remove the lifecycle columns added to incidents.
-- Does NOT drop the table (it was created by 006, not this migration).
ALTER TABLE incidents DROP COLUMN IF EXISTS acknowledged_at;
ALTER TABLE incidents DROP COLUMN IF EXISTS closed_at;
ALTER TABLE incidents DROP COLUMN IF EXISTS assigned_to;
ALTER TABLE incidents DROP COLUMN IF EXISTS escalated_at;
ALTER TABLE incidents DROP COLUMN IF EXISTS escalation_target;
ALTER TABLE incidents DROP COLUMN IF EXISTS description;

DROP INDEX IF EXISTS idx_incidents_status;
DROP INDEX IF EXISTS idx_incidents_created_at;
DROP INDEX IF EXISTS idx_incidents_assigned_to;
