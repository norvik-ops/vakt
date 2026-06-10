-- S68-4: Internes Audit-Programm (ISO 27001 Clause 9.2)
-- Structured audit program with plans, individual audits, and findings.

CREATE TABLE IF NOT EXISTS ck_audit_plans (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    year           INTEGER     NOT NULL,
    scope          TEXT,
    responsible_id UUID        REFERENCES users(id),
    status         TEXT        NOT NULL CHECK (status IN ('draft', 'approved', 'in_progress', 'completed'))
                               DEFAULT 'draft',
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, year)
);

CREATE TABLE IF NOT EXISTS ck_audit_program_audits (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    audit_plan_id   UUID        REFERENCES ck_audit_plans(id) ON DELETE SET NULL,
    title           TEXT        NOT NULL,
    audit_type      TEXT        NOT NULL CHECK (audit_type IN (
        'isms_internal', 'compliance_check', 'supplier_audit', 'process_audit'
    )),
    scope           TEXT        NOT NULL,
    methodology     TEXT        CHECK (methodology IN (
        'document_review', 'interview', 'technical_check', 'combined'
    )) DEFAULT 'combined',
    planned_date    DATE        NOT NULL,
    actual_date     DATE,
    lead_auditor_id UUID        REFERENCES users(id),
    auditor_ids     UUID[]      DEFAULT '{}',
    supplier_id     UUID,
    status          TEXT        NOT NULL CHECK (status IN (
        'planned', 'in_progress', 'completed', 'cancelled'
    )) DEFAULT 'planned',
    audit_report    TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ck_audit_program_findings (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    audit_id            UUID        NOT NULL REFERENCES ck_audit_program_audits(id) ON DELETE CASCADE,
    title               TEXT        NOT NULL,
    description         TEXT        NOT NULL,
    severity            TEXT        NOT NULL CHECK (severity IN ('major_nc', 'minor_nc', 'observation', 'ofi')),
    affected_control_id UUID        REFERENCES ck_controls(id) ON DELETE SET NULL,
    capa_id             UUID        REFERENCES ck_capas(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ck_audit_plans_org        ON ck_audit_plans (org_id, year);
CREATE INDEX IF NOT EXISTS idx_ck_audit_prog_audits_org  ON ck_audit_program_audits (org_id, status);
CREATE INDEX IF NOT EXISTS idx_ck_audit_prog_audits_plan ON ck_audit_program_audits (audit_plan_id);
CREATE INDEX IF NOT EXISTS idx_ck_audit_prog_findings    ON ck_audit_program_findings (audit_id);
