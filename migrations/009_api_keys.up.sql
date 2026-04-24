-- Machine-identity credentials. Separate from user JWTs.
-- Key value is shown once at creation; only sha256(key) is persisted.

CREATE TABLE api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id      UUID REFERENCES apps(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    key_prefix  VARCHAR(12) NOT NULL,
    key_hash    VARCHAR(64) NOT NULL UNIQUE,
    scopes      TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMP,
    expires_at  TIMESTAMP,
    revoked_at  TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_user ON api_keys(user_id);
