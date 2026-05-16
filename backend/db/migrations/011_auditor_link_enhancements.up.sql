-- E09: Auditor Link Enhancements
-- Adds multi-framework support, label, and access tracking columns to ck_auditor_links.

-- Add label for human-readable identification
ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS label TEXT NOT NULL DEFAULT '';

-- Add JSONB column for multiple framework IDs
ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS framework_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Add access tracking columns
ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS last_accessed_at TIMESTAMPTZ;

ALTER TABLE ck_auditor_links
    ADD COLUMN IF NOT EXISTS access_count INT NOT NULL DEFAULT 0;

-- Make framework_id nullable (existing rows keep their value; new multi-framework links use framework_ids)
ALTER TABLE ck_auditor_links
    ALTER COLUMN framework_id DROP NOT NULL;

-- Backfill framework_ids from existing framework_id values
UPDATE ck_auditor_links
SET framework_ids = jsonb_build_array(framework_id::text)
WHERE framework_id IS NOT NULL AND framework_ids = '[]'::jsonb;

-- Create index for faster token lookups (already unique, but explicit for clarity)
CREATE INDEX IF NOT EXISTS idx_ck_auditor_links_org ON ck_auditor_links(org_id);
