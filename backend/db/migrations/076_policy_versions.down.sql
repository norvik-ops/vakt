DROP TABLE IF EXISTS ck_policy_versions;
ALTER TABLE ck_policies
  DROP COLUMN IF EXISTS version_num,
  DROP COLUMN IF EXISTS version_note,
  DROP COLUMN IF EXISTS last_updated_by,
  DROP COLUMN IF EXISTS reviewed_at,
  DROP COLUMN IF EXISTS next_review_due;
