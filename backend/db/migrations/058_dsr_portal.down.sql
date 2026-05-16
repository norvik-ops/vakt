-- 058: DSR Self-Service Portal — rollback
DROP INDEX IF EXISTS idx_po_dsr_token;

ALTER TABLE po_dsr
    DROP COLUMN IF EXISTS token_hash,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS portal_locale,
    DROP COLUMN IF EXISTS submitted_ip,
    DROP COLUMN IF EXISTS verify_token_hash;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS dsr_portal_enabled,
    DROP COLUMN IF EXISTS dsr_portal_slug,
    DROP COLUMN IF EXISTS dsr_dpo_email,
    DROP COLUMN IF EXISTS dsr_portal_intro;
