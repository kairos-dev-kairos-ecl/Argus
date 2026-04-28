-- 020_add_api_key_user_fields.down.sql
DROP INDEX IF EXISTS idx_api_keys_user_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS user_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_prefix;
