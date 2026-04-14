-- Migration 015: Create notification_channels and update routing_rules for alert notification routing
-- Establishes the foundation for multi-channel alert dispatch (Slack, Email, PagerDuty, webhook, syslog)

CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('slack', 'email', 'pagerduty', 'webhook', 'syslog')),
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by VARCHAR(255) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_enabled ON notification_channels(enabled);
CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON notification_channels(type);

-- Add FK constraint from routing_rules.channel_id to notification_channels now that the table exists
ALTER TABLE routing_rules
ADD CONSTRAINT fk_routing_rules_channel
    FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE SET NULL;
