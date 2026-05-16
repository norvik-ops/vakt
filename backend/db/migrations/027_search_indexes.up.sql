-- Text search indexes on commonly searched columns
CREATE INDEX IF NOT EXISTS idx_vb_assets_name   ON vb_assets(org_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_vb_findings_title ON vb_findings(org_id, lower(title));
CREATE INDEX IF NOT EXISTS idx_po_dsr_name       ON po_dsr(org_id, lower(requester_name));
CREATE INDEX IF NOT EXISTS idx_ck_risks_title    ON ck_risks(org_id, lower(title));
CREATE INDEX IF NOT EXISTS idx_po_breaches_title ON po_breaches(org_id, lower(title));
