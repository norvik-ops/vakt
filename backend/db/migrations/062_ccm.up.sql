CREATE TYPE ck_check_type AS ENUM ('http_endpoint', 'trivy_no_critical', 'evidence_freshness', 'custom_script');

CREATE TABLE IF NOT EXISTS ck_ccm_checks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    control_id      UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    check_type      ck_check_type NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    interval_hours  INT NOT NULL DEFAULT 24,
    last_run_at     TIMESTAMPTZ,
    last_status     TEXT CHECK (last_status IN ('pass', 'fail', 'unknown')),
    last_output     TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ck_ccm_checks_org_idx ON ck_ccm_checks(org_id);
CREATE INDEX IF NOT EXISTS ck_ccm_checks_control_idx ON ck_ccm_checks(control_id);
CREATE INDEX IF NOT EXISTS ck_ccm_checks_next_run_idx ON ck_ccm_checks(last_run_at, enabled) WHERE enabled = true;

CREATE TABLE IF NOT EXISTS ck_ccm_results (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id    UUID NOT NULL REFERENCES ck_ccm_checks(id) ON DELETE CASCADE,
    status      TEXT NOT NULL CHECK (status IN ('pass', 'fail', 'unknown')),
    output      TEXT,
    ran_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ck_ccm_results_check_idx ON ck_ccm_results(check_id, ran_at DESC);
