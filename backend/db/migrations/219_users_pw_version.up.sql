-- 219_users_pw_version.up.sql
-- S87-6 (F-06, CWE-636): persist the password version counter in PostgreSQL so
-- that checkPwVersion can fall back to PG when Redis is unavailable instead of
-- failing open. Redis remains the hot path; PG is the durable source of truth.
-- Default 0 matches the "key absent ⇒ version 0" semantics in Redis.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS pw_version BIGINT NOT NULL DEFAULT 0;
