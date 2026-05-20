-- 121 down
DROP INDEX IF EXISTS idx_ck_auditor_links_active;
ALTER TABLE ck_auditor_links DROP COLUMN IF EXISTS revoked_at;
