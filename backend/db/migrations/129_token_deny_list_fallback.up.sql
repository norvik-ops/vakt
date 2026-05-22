-- Migration 129: PostgreSQL fallback table for token deny list.
-- When Redis is unavailable, revoked tokens are written here so that
-- AuthMiddleware can still reject them (S31-4: Redis-SPOF entschärfen).
CREATE TABLE IF NOT EXISTS token_deny_list_fallback (
    token_hash  TEXT        PRIMARY KEY,
    expires_at  TIMESTAMPTZ NOT NULL
);

-- GIN-style partial index: only live entries are ever checked.
CREATE INDEX IF NOT EXISTS idx_token_deny_fallback_expires
    ON token_deny_list_fallback (expires_at)
    WHERE expires_at > NOW();
