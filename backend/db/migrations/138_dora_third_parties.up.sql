-- S38-1: DORA IKT-Drittanbieter-Register (Art. 28-44).
CREATE TABLE dora_third_parties (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    service_type        TEXT NOT NULL,                        -- IT-Outsourcing | Cloud | SaaS | Netzwerk | Sonstiges
    criticality         TEXT NOT NULL DEFAULT 'wichtig'
        CHECK (criticality IN ('kritisch', 'wichtig', 'unkritisch')),
    contract_start      DATE NULL,
    contract_end        DATE NULL,
    sla_rto_hours       INT NULL,                             -- Recovery Time Objective
    sla_availability    NUMERIC(5,2) NULL,                    -- z.B. 99.9 für 99.9%
    has_subcontractors  BOOLEAN NOT NULL DEFAULT false,
    subcontractor_names TEXT NULL,
    data_location       TEXT NOT NULL DEFAULT 'EU'
        CHECK (data_location IN ('EU', 'Non-EU', 'Mixed')),
    exit_strategy       BOOLEAN NOT NULL DEFAULT false,       -- Ausstiegsstrategie vorhanden?
    exit_notes          TEXT NULL,
    notes               TEXT NULL,
    created_by          UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dora_third_parties_org ON dora_third_parties (org_id);
CREATE INDEX idx_dora_third_parties_criticality ON dora_third_parties (org_id, criticality);

-- Link third parties to DORA controls in Vakt Comply.
CREATE TABLE dora_third_party_controls (
    third_party_id  UUID NOT NULL REFERENCES dora_third_parties(id) ON DELETE CASCADE,
    control_id      UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    PRIMARY KEY (third_party_id, control_id)
);
