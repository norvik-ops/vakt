ALTER TABLE ck_auditor_links
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS allowed_frameworks;
