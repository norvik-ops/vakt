-- 219_users_pw_version.down.sql
ALTER TABLE users DROP COLUMN IF EXISTS pw_version;
