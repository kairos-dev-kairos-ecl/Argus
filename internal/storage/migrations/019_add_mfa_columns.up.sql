-- Add MFA columns to users table (Phase 6)
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS mfa_enabled          BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS mfa_secret_encrypted TEXT;
