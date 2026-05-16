-- KI-System-Inventar: AI system inventory for EU AI Act compliance.
CREATE TABLE ck_ai_systems (
  id                        UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id                    UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name                      TEXT        NOT NULL,
  description               TEXT,
  provider                  TEXT,
  use_case                  TEXT,
  affected_groups           TEXT,
  autonomy_level            TEXT        NOT NULL DEFAULT 'assistive',
  in_production_since       DATE,
  status                    TEXT        NOT NULL DEFAULT 'under_review',
  risk_class                TEXT,
  classification_rationale  TEXT,
  classified_at             TIMESTAMPTZ,
  classified_by             TEXT,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_ai_systems_org_id ON ck_ai_systems (org_id);
