CREATE TABLE ck_isms_kpi_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    snapshot_date DATE NOT NULL,
    kpi_compliance_score NUMERIC(5,2),
    kpi_open_critical_controls INTEGER,
    kpi_open_high_risks INTEGER,
    kpi_residual_risk_avg NUMERIC(5,2),
    kpi_open_incidents INTEGER,
    kpi_incident_mttr_days NUMERIC(5,1),
    kpi_evidence_coverage NUMERIC(5,2),
    kpi_expiring_evidence_count INTEGER,
    kpi_finding_sla_compliance NUMERIC(5,2),
    kpi_open_major_ncs INTEGER,
    kpi_suppliers_overdue_pct NUMERIC(5,2),
    kpi_phishing_click_rate NUMERIC(5,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, snapshot_date)
);
CREATE INDEX idx_ck_isms_kpi_snapshots_org ON ck_isms_kpi_snapshots (org_id, snapshot_date DESC);
