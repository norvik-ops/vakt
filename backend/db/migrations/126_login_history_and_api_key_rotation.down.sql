DROP TABLE IF EXISTS login_history;
ALTER TABLE api_keys
    DROP COLUMN IF EXISTS rotated_at,
    DROP COLUMN IF EXISTS last_used_ip,
    DROP COLUMN IF EXISTS previous_key_grace_expires_at,
    DROP COLUMN IF EXISTS previous_key_hash;
