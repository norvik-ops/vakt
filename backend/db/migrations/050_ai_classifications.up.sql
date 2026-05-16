-- AI classification history for EU AI Act risk classification wizard.
CREATE TABLE ck_ai_classifications (
  id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  ai_system_id    UUID        NOT NULL REFERENCES ck_ai_systems(id) ON DELETE CASCADE,
  risk_class      TEXT        NOT NULL,
  rationale       TEXT,
  classified_by   TEXT,
  wizard_answers  JSONB,
  classified_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_ai_classifications_ai_system_id ON ck_ai_classifications (ai_system_id);
