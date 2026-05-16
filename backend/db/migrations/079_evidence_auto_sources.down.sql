ALTER TABLE ck_evidence
  DROP COLUMN IF EXISTS auto_source_type,
  DROP COLUMN IF EXISTS auto_source_ref,
  DROP COLUMN IF EXISTS auto_collected_at;

-- Restore NOT NULL constraint (only if all rows have control_id set).
ALTER TABLE ck_evidence ALTER COLUMN control_id SET NOT NULL;
