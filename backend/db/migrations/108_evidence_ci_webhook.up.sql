-- Migration 108: Expand auto_source_type CHECK constraint to include CI pipeline types.
-- Adds 'ci_pipeline' (GitHub Actions pull model) and 'ci_webhook' (generic push webhook).

ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;

ALTER TABLE ck_evidence
  ADD CONSTRAINT ck_evidence_auto_source_type_check
  CHECK (auto_source_type IN ('github', 'vaktaware', 'vaktscan', 'ci_pipeline', 'ci_webhook'));
