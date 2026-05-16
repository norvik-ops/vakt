-- PrivacyOps: DSGVO documentation hub
-- VVT (Art.30), DPIA (Art.35), AVV (Art.28), Breach Notifications (Art.33/34)

CREATE TABLE po_vvt_entries (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                 UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                   TEXT        NOT NULL,
    purpose                TEXT        NOT NULL,
    legal_basis            TEXT        NOT NULL,
    data_categories        TEXT[]      NOT NULL DEFAULT '{}',
    data_subjects          TEXT[]      NOT NULL DEFAULT '{}',
    recipients             TEXT[]      NOT NULL DEFAULT '{}',
    retention_period       TEXT,
    third_country_transfer BOOLEAN     NOT NULL DEFAULT false,
    safeguards             TEXT,
    responsible_person     TEXT,
    status                 TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE po_dpias (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                 UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    vvt_entry_id           UUID        REFERENCES po_vvt_entries(id) ON DELETE SET NULL,
    title                  TEXT        NOT NULL,
    description            TEXT,
    necessity_assessment   TEXT,
    risk_assessment        TEXT,
    mitigation_measures    TEXT,
    residual_risk          TEXT,
    dpo_consultation       BOOLEAN     NOT NULL DEFAULT false,
    status                 TEXT        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','in_review','approved')),
    reviewed_by            UUID        REFERENCES users(id),
    reviewed_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE po_avvs (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id               UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    processor_name       TEXT        NOT NULL,
    service_description  TEXT        NOT NULL,
    contract_date        DATE,
    review_date          DATE,
    status               TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','expired','terminated')),
    notes                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE po_breaches (
    id                               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                           UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title                            TEXT        NOT NULL,
    description                      TEXT        NOT NULL,
    discovered_at                    TIMESTAMPTZ NOT NULL,
    authority_deadline_at            TIMESTAMPTZ NOT NULL,
    authority_notified_at            TIMESTAMPTZ,
    subjects_notification_required   BOOLEAN     NOT NULL DEFAULT false,
    subjects_notified_at             TIMESTAMPTZ,
    affected_count                   INTEGER,
    data_categories                  TEXT[]      NOT NULL DEFAULT '{}',
    status                           TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open','authority_notified','closed')),
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_po_vvt_entries_org_id ON po_vvt_entries(org_id);
CREATE INDEX idx_po_dpias_org_id       ON po_dpias(org_id);
CREATE INDEX idx_po_avvs_org_id        ON po_avvs(org_id);
CREATE INDEX idx_po_breaches_org_id    ON po_breaches(org_id);
