CREATE TABLE ck_control_exceptions (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id       UUID NOT NULL,
  control_id   UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  title        TEXT NOT NULL,
  reason       TEXT NOT NULL,
  risk_accepted TEXT NOT NULL,
  approved_by  TEXT,
  expires_at   TIMESTAMPTZ,
  status       TEXT NOT NULL DEFAULT 'active',
  created_by   TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON ck_control_exceptions(control_id, org_id);
