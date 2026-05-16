-- Add versioning columns to existing ck_policies table
ALTER TABLE ck_policies
  ADD COLUMN IF NOT EXISTS version_num INT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS version_note TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_updated_by TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS next_review_due DATE;

-- Version history table
CREATE TABLE IF NOT EXISTS ck_policy_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  policy_id UUID NOT NULL REFERENCES ck_policies(id) ON DELETE CASCADE,
  version INT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  version_note TEXT NOT NULL DEFAULT '',
  updated_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ck_pv_policy ON ck_policy_versions(policy_id, version DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ck_pv_unique ON ck_policy_versions(policy_id, version);
