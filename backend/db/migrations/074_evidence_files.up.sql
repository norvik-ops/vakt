CREATE TABLE IF NOT EXISTS ck_evidence_files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  evidence_id UUID REFERENCES ck_evidence(id) ON DELETE CASCADE,
  control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  original_name TEXT NOT NULL,
  stored_name TEXT NOT NULL,
  mime_type TEXT NOT NULL DEFAULT '',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  uploaded_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ck_ef_evidence ON ck_evidence_files(evidence_id);
CREATE INDEX IF NOT EXISTS idx_ck_ef_control ON ck_evidence_files(org_id, control_id);
