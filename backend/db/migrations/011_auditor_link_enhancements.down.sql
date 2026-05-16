-- Revert E09 auditor link enhancements

DROP INDEX IF EXISTS idx_ck_auditor_links_org;

ALTER TABLE ck_auditor_links
    ALTER COLUMN framework_id SET NOT NULL;

ALTER TABLE ck_auditor_links
    DROP COLUMN IF EXISTS access_count;

ALTER TABLE ck_auditor_links
    DROP COLUMN IF EXISTS last_accessed_at;

ALTER TABLE ck_auditor_links
    DROP COLUMN IF EXISTS framework_ids;

ALTER TABLE ck_auditor_links
    DROP COLUMN IF EXISTS label;
