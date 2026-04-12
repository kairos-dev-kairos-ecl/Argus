-- 004_create_detection_rules.up.sql
CREATE TABLE IF NOT EXISTS detection_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id     TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    tier        TEXT NOT NULL,        -- 'deterministic', 'statistical', 'temporal', 'semantic'
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    yaml_config TEXT NOT NULL,        -- raw YAML rule definition
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version     INT NOT NULL DEFAULT 1
);
