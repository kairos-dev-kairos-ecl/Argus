-- Wave 4: MFA Primitives
-- Schema additions for TOTP-based multi-factor authentication and backup codes

-- Extend users table with MFA-related fields
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS mfa_enabled BOOL NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS mfa_secret_encrypted TEXT;

-- Create user_backup_codes table for storing hashed backup codes
CREATE TABLE IF NOT EXISTS user_backup_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash VARCHAR(64) NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

-- Index for efficient queries of unused backup codes by user
CREATE INDEX IF NOT EXISTS idx_backup_codes_user ON user_backup_codes(user_id) WHERE used_at IS NULL;
