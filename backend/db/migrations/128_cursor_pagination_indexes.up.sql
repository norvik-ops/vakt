-- Cursor-based pagination indexes for Welle 1 + 2 endpoints.
-- Composite (created_at DESC, id DESC) supports keyset pagination:
--   WHERE (created_at, id::text) < (cursor_ts, cursor_id)
--   ORDER BY created_at DESC, id DESC

-- secpulse findings (table is vb_findings)
CREATE INDEX IF NOT EXISTS idx_vb_findings_cursor
    ON vb_findings (org_id, created_at DESC, id DESC);

-- secvitals risks
CREATE INDEX IF NOT EXISTS idx_ck_risks_cursor
    ON ck_risks (org_id, created_at DESC, id DESC);

-- secvitals controls (ordered by control_id ASC for keyset)
CREATE INDEX IF NOT EXISTS idx_ck_controls_cursor
    ON ck_controls (framework_id, control_id ASC, id ASC);

-- secvault secrets
CREATE INDEX IF NOT EXISTS idx_so_secrets_cursor
    ON so_secrets (project_id, created_at DESC, id DESC);

-- secprivacy DSRs (table is po_dsr)
CREATE INDEX IF NOT EXISTS idx_po_dsr_cursor
    ON po_dsr (org_id, created_at DESC, id DESC);

-- hr employees
CREATE INDEX IF NOT EXISTS idx_hr_employees_cursor
    ON hr_employees (org_id, created_at DESC, id DESC);

-- secreflex campaigns (table is sr_campaigns)
CREATE INDEX IF NOT EXISTS idx_sr_campaigns_cursor
    ON sr_campaigns (org_id, created_at DESC, id DESC);
