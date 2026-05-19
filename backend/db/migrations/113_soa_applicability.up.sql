-- 103_soa_applicability.up.sql
ALTER TABLE ck_controls
  ADD COLUMN IF NOT EXISTS soa_applicable          BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS soa_justification_yes   TEXT,
  ADD COLUMN IF NOT EXISTS soa_justification_no    TEXT;
