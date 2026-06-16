-- S86-1: BSI-200-4 Wiederanlaufpläne
CREATE TABLE ck_recovery_plans (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID NOT NULL,
    bia_process_id       UUID REFERENCES ck_bia_processes(id) ON DELETE SET NULL,
    title                TEXT NOT NULL,
    activation_criteria  TEXT NOT NULL DEFAULT '',
    responsible          TEXT NOT NULL DEFAULT '',
    rto_hours            INT  NOT NULL DEFAULT 72,
    status               TEXT NOT NULL DEFAULT 'draft'
                             CHECK (status IN ('draft','active','tested','archived')),
    steps                JSONB NOT NULL DEFAULT '[]',
    last_tested_at       DATE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_recovery_plans_org_id ON ck_recovery_plans (org_id);
CREATE INDEX idx_ck_recovery_plans_bia_id ON ck_recovery_plans (bia_process_id);
