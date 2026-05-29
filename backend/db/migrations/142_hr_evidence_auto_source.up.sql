-- Migration 142: Add 'hr' to auto_source_type so HR checklist completions
-- appear in the EvidenceAutoPage alongside GitHub, Scan, and Aware evidence.

ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;

ALTER TABLE ck_evidence
  ADD CONSTRAINT ck_evidence_auto_source_type_check
  CHECK (auto_source_type IN ('github', 'vaktaware', 'vaktscan', 'ci_pipeline', 'ci_webhook', 'hr'));

-- Backfill: set auto_source_type='hr' on existing HR checklist completions
-- that were inserted before this migration (source = 'hr_checklist_completed').
UPDATE ck_evidence
   SET auto_source_type = 'hr',
       auto_collected_at = COALESCE(auto_collected_at, created_at)
 WHERE source = 'hr_checklist_completed'
   AND auto_source_type IS NULL;
