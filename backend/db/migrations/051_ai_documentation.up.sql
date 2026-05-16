-- AI technical documentation for EU AI Act Art. 11 / Annex IV compliance.
CREATE TABLE ck_ai_documentation (
  id                    UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  ai_system_id          UUID        NOT NULL REFERENCES ck_ai_systems(id) ON DELETE CASCADE,
  version               INT         NOT NULL DEFAULT 1,
  -- Art. 11 / Annex IV fields
  system_description    TEXT,
  intended_purpose      TEXT,
  training_data         TEXT,
  data_quality          TEXT,
  performance_metrics   TEXT,
  system_limits         TEXT,
  risk_management       TEXT,
  human_oversight       TEXT,
  logging_audit_trail   TEXT,
  -- Metadata
  authored_by           TEXT,
  status                TEXT        NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft','final')),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_ai_documentation_ai_system_id ON ck_ai_documentation (ai_system_id);
