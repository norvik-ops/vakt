-- S68-3: Interessierte Parteien (ISO 27001 Clause 4.2)
-- Stakeholder register for ISMS scope documentation.

CREATE TABLE IF NOT EXISTS ck_interested_parties (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name              TEXT        NOT NULL,
    category          TEXT        NOT NULL CHECK (category IN (
        'customer', 'regulator', 'employee', 'shareholder',
        'supplier', 'insurer', 'it_provider', 'other'
    )),
    requirements      TEXT,
    concerns          TEXT,
    review_date       DATE,
    is_system_default BOOLEAN     NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ck_interested_parties_org ON ck_interested_parties (org_id);
