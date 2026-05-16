-- Allow control_id to be NULL for auto-collected evidence not yet assigned to a control.
ALTER TABLE ck_evidence ALTER COLUMN control_id DROP NOT NULL;

ALTER TABLE ck_evidence
  ADD COLUMN IF NOT EXISTS auto_source_type TEXT CHECK (auto_source_type IN ('github', 'secreflex', 'secpulse')),
  ADD COLUMN IF NOT EXISTS auto_source_ref  TEXT,
  ADD COLUMN IF NOT EXISTS auto_collected_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS ck_evidence_auto_source_idx ON ck_evidence (org_id, auto_source_type) WHERE auto_source_type IS NOT NULL;
