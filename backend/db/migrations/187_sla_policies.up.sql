-- S69-3: SLA-Enforcement für Vulnerability Findings
-- Per-severity SLA policies with notification advance and automatic due-date tracking.

CREATE TABLE IF NOT EXISTS vb_sla_policies (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    severity                    TEXT        NOT NULL CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info')),
    remediation_days            INTEGER     NOT NULL,
    notification_advance_days   INTEGER     NOT NULL DEFAULT 3,
    is_default                  BOOLEAN     NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, severity)
);

CREATE INDEX IF NOT EXISTS idx_vb_sla_policies_org ON vb_sla_policies (org_id);

-- Extend vb_findings with SLA tracking columns
ALTER TABLE vb_findings
    ADD COLUMN IF NOT EXISTS sla_due_at          TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sla_status          TEXT
        CHECK (sla_status IN ('on_track', 'at_risk', 'overdue', 'resolved_on_time', 'resolved_late'))
        DEFAULT 'on_track',
    ADD COLUMN IF NOT EXISTS sla_resolved_within BOOLEAN,
    ADD COLUMN IF NOT EXISTS sla_actual_days     INTEGER;

CREATE INDEX IF NOT EXISTS idx_vb_findings_sla
    ON vb_findings (org_id, sla_status, sla_due_at)
    WHERE sla_status NOT IN ('resolved_on_time', 'resolved_late');
