-- Risk Assessment (FR-CK12)
CREATE TABLE ck_risks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT '',
    likelihood      SMALLINT NOT NULL CHECK (likelihood BETWEEN 1 AND 5),
    impact          SMALLINT NOT NULL CHECK (impact BETWEEN 1 AND 5),
    risk_score      SMALLINT GENERATED ALWAYS AS (likelihood * impact) STORED,
    owner           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','mitigated','accepted','closed')),
    treatment       TEXT NOT NULL DEFAULT 'mitigate' CHECK (treatment IN ('avoid','mitigate','transfer','accept')),
    treatment_notes TEXT NOT NULL DEFAULT '',
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ck_risks_org_id ON ck_risks (org_id);

-- Incident Register (FR-CK13)
CREATE TABLE ck_incidents (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    severity         TEXT NOT NULL DEFAULT 'medium' CHECK (severity IN ('low','medium','high','critical')),
    status           TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','investigating','resolved','closed')),
    discovered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at      TIMESTAMPTZ,
    affected_systems TEXT[] NOT NULL DEFAULT '{}',
    breach_id        UUID REFERENCES po_breaches(id) ON DELETE SET NULL,
    created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ck_incidents_org_id ON ck_incidents (org_id);

-- Policy Management (FR-CK14)
CREATE TABLE ck_policies (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    category       TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','archived')),
    version        TEXT NOT NULL DEFAULT '1.0',
    effective_date DATE,
    review_date    DATE,
    owner          TEXT NOT NULL DEFAULT '',
    created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ck_policies_org_id ON ck_policies (org_id);

-- Internal Audit Records (FR-CK15)
CREATE TABLE ck_audit_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    scope           TEXT NOT NULL DEFAULT '',
    auditor         TEXT NOT NULL DEFAULT '',
    audit_date      DATE NOT NULL,
    status          TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','in_progress','completed')),
    findings        TEXT NOT NULL DEFAULT '',
    recommendations TEXT NOT NULL DEFAULT '',
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ck_audit_records_org_id ON ck_audit_records (org_id);
