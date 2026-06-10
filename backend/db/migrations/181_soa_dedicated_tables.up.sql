-- S68-1: Statement of Applicability (ISO 27001 Clause 6.1.3)
-- Dedicated ck_soa_entries and ck_soa_versions tables for full versioning + approval.

CREATE TABLE IF NOT EXISTS ck_soa_versions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version      INTEGER     NOT NULL,
    status       TEXT        NOT NULL CHECK (status IN ('draft', 'approved')) DEFAULT 'draft',
    approved_by  UUID        REFERENCES users(id),
    approved_at  TIMESTAMPTZ,
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, version)
);

CREATE TABLE IF NOT EXISTS ck_soa_entries (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version               INTEGER     NOT NULL DEFAULT 1,
    control_ref           TEXT        NOT NULL,
    control_name          TEXT        NOT NULL,
    control_group         TEXT        NOT NULL CHECK (control_group IN ('5', '6', '7', '8')),
    applicable            BOOLEAN     NOT NULL DEFAULT true,
    justification         TEXT,
    exclusion_reason      TEXT,
    implementation_status TEXT        NOT NULL
        CHECK (implementation_status IN ('not_started', 'planned', 'partial', 'implemented'))
        DEFAULT 'not_started',
    manually_set          BOOLEAN     NOT NULL DEFAULT false,
    ck_control_id         UUID        REFERENCES ck_controls(id) ON DELETE SET NULL,
    evidence_reference    TEXT,
    is_approved           BOOLEAN     NOT NULL DEFAULT false,
    approved_by           UUID        REFERENCES users(id),
    approved_at           TIMESTAMPTZ,
    notes                 TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, version, control_ref)
);

CREATE INDEX IF NOT EXISTS idx_ck_soa_entries_org ON ck_soa_entries (org_id, version, control_group);
CREATE INDEX IF NOT EXISTS idx_ck_soa_versions_org ON ck_soa_versions (org_id, version);
