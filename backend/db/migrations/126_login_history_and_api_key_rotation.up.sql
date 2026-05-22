-- Sprint 20 / S20-2 + S20-6: Enterprise-Auth CE-Tier.
--
-- S20-2: API-Key-Rotation mit Grace-Period.
--   `previous_key_hash` + `previous_key_grace_expires_at` halten den alten
--   Hash für 24 h nach Rotation parallel — beide Hashes werden vom Auth-
--   Middleware akzeptiert, alter Hash erhält X-Vakt-Key-Deprecated-Header
--   und Sunset-Datum als Hinweis für die CI-Pipeline.
--
-- S20-6: login_history-Tabelle.
--   Pro Login-Versuch (auch failed) ein Eintrag mit IP, User-Agent, Quelle
--   (password|oidc|magic_link), Result-Code. 90-Tage-Retention via Cleanup-
--   Job. Hilft Customer beim Audit von Account-Übernahme-Verdachtsfällen.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS previous_key_hash TEXT,
    ADD COLUMN IF NOT EXISTS previous_key_grace_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_used_ip TEXT,
    ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS login_history (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT,                                       -- bei failed-login ohne User-ID
    ts         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip         TEXT,
    user_agent TEXT,
    source     TEXT NOT NULL,                              -- 'password' | 'oidc' | 'magic_link' | 'api_key'
    result     TEXT NOT NULL                               -- 'ok' | 'bad_password' | 'locked' | 'mfa_failed' | 'oidc_failed'
);

CREATE INDEX IF NOT EXISTS idx_login_history_user_ts
    ON login_history (user_id, ts DESC);

CREATE INDEX IF NOT EXISTS idx_login_history_email_ts
    ON login_history (email, ts DESC)
    WHERE user_id IS NULL;
