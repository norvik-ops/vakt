-- 058: DSR Self-Service Portal — public submission without login
ALTER TABLE po_dsr
    ADD COLUMN IF NOT EXISTS token_hash        TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS source            TEXT NOT NULL DEFAULT 'internal'
        CHECK (source IN ('internal', 'portal')),
    ADD COLUMN IF NOT EXISTS portal_locale     TEXT DEFAULT 'de',
    ADD COLUMN IF NOT EXISTS submitted_ip      TEXT,
    ADD COLUMN IF NOT EXISTS verify_token_hash TEXT UNIQUE;
CREATE INDEX IF NOT EXISTS idx_po_dsr_token ON po_dsr(token_hash) WHERE token_hash IS NOT NULL;

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS dsr_portal_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS dsr_portal_slug     TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS dsr_dpo_email       TEXT,
    ADD COLUMN IF NOT EXISTS dsr_portal_intro    TEXT;
