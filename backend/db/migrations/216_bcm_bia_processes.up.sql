-- S86-1: BSI-200-4 Business Impact Analysis
CREATE TABLE ck_bia_processes (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL,
    name                  TEXT NOT NULL,
    description           TEXT NOT NULL DEFAULT '',
    process_owner         TEXT NOT NULL DEFAULT '',
    criticality           TEXT NOT NULL DEFAULT 'medium'
                              CHECK (criticality IN ('high','medium','low')),
    schutzbedarfsklasse   INT  NOT NULL DEFAULT 2
                              CHECK (schutzbedarfsklasse IN (1,2,3)),
    rto_hours             INT  NOT NULL DEFAULT 72,
    rpo_hours             INT  NOT NULL DEFAULT 24,
    mbco_percent          INT  NOT NULL DEFAULT 50
                              CHECK (mbco_percent BETWEEN 0 AND 100),
    dependencies          TEXT[] NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_bia_processes_org_id ON ck_bia_processes (org_id);

-- Extend ck_bcp_plans with BSI-200-4 fields
ALTER TABLE ck_bcp_plans
    ADD COLUMN IF NOT EXISTS rto_hours           INT  NOT NULL DEFAULT 72,
    ADD COLUMN IF NOT EXISTS rpo_hours           INT  NOT NULL DEFAULT 24,
    ADD COLUMN IF NOT EXISTS schutzbedarfsklasse INT  NOT NULL DEFAULT 2
                                                     CHECK (schutzbedarfsklasse IN (1,2,3)),
    ADD COLUMN IF NOT EXISTS last_tested_at      DATE;
