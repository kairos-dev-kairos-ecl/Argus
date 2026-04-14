-- Rollback Migration 015

-- Drop columns from routing_rules
ALTER TABLE routing_rules
DROP COLUMN IF EXISTS channel_id,
DROP COLUMN IF EXISTS min_severity,
DROP COLUMN IF EXISTS app_id_filter,
DROP COLUMN IF EXISTS layer_filter;

-- Drop notification_channels table
DROP TABLE IF EXISTS notification_channels;
