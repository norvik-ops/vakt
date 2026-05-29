-- Revert migration 142: Remove 'hr' from auto_source_type CHECK constraint.
-- Note: existing HR evidence rows will have auto_source_type cleared.

UPDATE ck_evidence
   SET auto_source_type = NULL
 WHERE auto_source_type = 'hr';

ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;

ALTER TABLE ck_evidence
  ADD CONSTRAINT ck_evidence_auto_source_type_check
  CHECK (auto_source_type IN ('github', 'vaktaware', 'vaktscan', 'ci_pipeline', 'ci_webhook'));
