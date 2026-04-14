-- Apps: registered applications with API keys
CREATE TABLE IF NOT EXISTS apps (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    description    TEXT,
    api_key        TEXT NOT NULL UNIQUE,
    api_key_prefix TEXT NOT NULL,
    created_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked'))
);
CREATE INDEX IF NOT EXISTS idx_apps_api_key ON apps(api_key);
CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);

-- Detection rules
CREATE TABLE IF NOT EXISTS detection_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    tier        INTEGER NOT NULL CHECK (tier IN (1, 2, 3)),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    config      JSONB NOT NULL,  -- rule definition (authored as YAML, stored as JSONB)
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_detection_rules_enabled ON detection_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_detection_rules_tier ON detection_rules(tier);

-- Alerts
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID REFERENCES detection_rules(id),
    app_id          TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,
    severity        INTEGER NOT NULL CHECK (severity BETWEEN 1 AND 5),
    layer           INTEGER NOT NULL CHECK (layer BETWEEN 1 AND 10),
    category        TEXT NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    signal_ids      TEXT[] NOT NULL DEFAULT '{}',
    trace_id        TEXT,
    incident_id     UUID,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved', 'suppressed')),
    signal_count    INTEGER NOT NULL DEFAULT 1,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX IF NOT EXISTS idx_alerts_app_id ON alerts(app_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_trace_id ON alerts(trace_id);
CREATE INDEX IF NOT EXISTS idx_alerts_incident_id ON alerts(incident_id);
CREATE INDEX IF NOT EXISTS idx_alerts_first_seen ON alerts(first_seen_at DESC);

-- Incidents
CREATE TABLE IF NOT EXISTS incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT NOT NULL,
    description     TEXT,
    severity        INTEGER NOT NULL CHECK (severity BETWEEN 1 AND 5),
    app_id          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'resolved')),
    alert_ids       UUID[] NOT NULL DEFAULT '{}',
    trace_ids       TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_incidents_app_id ON incidents(app_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_created ON incidents(created_at DESC);

-- Notification channels
CREATE TABLE IF NOT EXISTS notification_channels (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (type IN ('slack', 'email', 'pagerduty', 'webhook', 'syslog')),
    config     JSONB NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Alert routing rules
CREATE TABLE IF NOT EXISTS routing_rules (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id    UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    min_severity  INTEGER NOT NULL DEFAULT 1 CHECK (min_severity BETWEEN 1 AND 5),
    app_id_filter TEXT,    -- NULL = match all apps
    layer_filter  INTEGER, -- NULL = match all layers
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Suppression rules (dedup windows)
CREATE TABLE IF NOT EXISTS suppression_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint      TEXT NOT NULL,
    suppressed_until TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_suppression_fingerprint ON suppression_rules(fingerprint);
CREATE INDEX IF NOT EXISTS idx_suppression_until ON suppression_rules(suppressed_until);
