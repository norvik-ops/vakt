-- 103_soa_applicability.down.sql
ALTER TABLE ck_controls
  DROP COLUMN IF EXISTS soa_applicable,
  DROP COLUMN IF EXISTS soa_justification_yes,
  DROP COLUMN IF EXISTS soa_justification_no;
