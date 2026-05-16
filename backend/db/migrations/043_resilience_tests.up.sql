CREATE TABLE IF NOT EXISTS ck_resilience_tests (
  id                  UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  type                TEXT        NOT NULL CHECK (type IN ('tlpt', 'pentest', 'scenario_based', 'vulnerability_assessment')),
  scope               TEXT,
  provider            TEXT,
  test_date           DATE        NOT NULL,
  summary             TEXT,
  remediation_status  TEXT        NOT NULL DEFAULT 'open' CHECK (remediation_status IN ('open', 'in_progress', 'completed', 'accepted')),
  attachment_url      TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ck_resilience_tests_org_id ON ck_resilience_tests (org_id);
CREATE INDEX IF NOT EXISTS idx_ck_resilience_tests_test_date ON ck_resilience_tests (org_id, test_date DESC);
