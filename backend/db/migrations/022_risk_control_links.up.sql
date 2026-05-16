CREATE TABLE IF NOT EXISTS ck_risk_control_links (
    risk_id    UUID NOT NULL REFERENCES ck_risks(id) ON DELETE CASCADE,
    control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (risk_id, control_id)
);
