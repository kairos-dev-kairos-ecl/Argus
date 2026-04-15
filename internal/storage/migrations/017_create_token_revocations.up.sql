-- 017_create_token_revocations.up.sql
CREATE TABLE IF NOT EXISTS token_revocations (
    token_hash  TEXT PRIMARY KEY,
    revoked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_token_revocations_expires_at ON token_revocations(expires_at);
