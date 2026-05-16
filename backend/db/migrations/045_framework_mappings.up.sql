CREATE TABLE IF NOT EXISTS ck_framework_mappings (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  source_control_id UUID        NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  target_control_id UUID        NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (org_id, source_control_id, target_control_id)
);
CREATE INDEX IF NOT EXISTS ck_framework_mappings_org_source ON ck_framework_mappings (org_id, source_control_id);
