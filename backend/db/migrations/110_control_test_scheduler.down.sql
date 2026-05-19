DROP INDEX IF EXISTS idx_ck_controls_next_test;

ALTER TABLE ck_controls
  DROP COLUMN IF EXISTS next_test_due_at,
  DROP COLUMN IF EXISTS last_tested_at,
  DROP COLUMN IF EXISTS test_interval_days;

-- Restore the original source_type CHECK constraint.
ALTER TABLE ck_capas
  DROP CONSTRAINT IF EXISTS ck_capas_source_type_check;
ALTER TABLE ck_capas
  ADD CONSTRAINT ck_capas_source_type_check
    CHECK (source_type IN ('audit','incident','risk','manual'));
