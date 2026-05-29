-- Revert Migration 108: Restore original auto_source_type CHECK constraint.

ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;

ALTER TABLE ck_evidence
  ADD CONSTRAINT ck_evidence_auto_source_type_check
  CHECK (auto_source_type IN ('github', 'vaktaware', 'vaktscan'));
