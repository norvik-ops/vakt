CREATE TABLE ck_bcp_plans (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    title       TEXT NOT NULL,
    scope       TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT '1.0',
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','archived')),
    owner       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_bcp_plans_org_id ON ck_bcp_plans (org_id);

CREATE TABLE ck_bcp_tests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    plan_id     UUID NOT NULL REFERENCES ck_bcp_plans(id) ON DELETE CASCADE,
    test_date   DATE NOT NULL,
    test_type   TEXT NOT NULL CHECK (test_type IN ('tabletop','walkthrough','fulltest')),
    outcome     TEXT NOT NULL CHECK (outcome IN ('passed','failed','partial')),
    findings    TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_bcp_tests_plan_id ON ck_bcp_tests (plan_id);
