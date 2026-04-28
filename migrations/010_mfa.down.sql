-- Reverse Wave 4: MFA Primitives
DROP TABLE IF EXISTS user_backup_codes;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_secret_encrypted;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;
