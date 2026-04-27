-- 020_add_api_key_user_fields.up.sql
-- Add user_id and key_prefix columns to api_keys table.
-- user_id tracks which user created the key (backfilled from created_by).
-- key_prefix stores the first 12 chars of the key for display.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS key_prefix VARCHAR(12) NOT NULL DEFAULT '';

-- Backfill user_id from created_by
UPDATE api_keys SET user_id = created_by WHERE created_by IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
