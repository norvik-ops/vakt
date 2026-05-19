-- 104_additional_indexes.up.sql
-- Additional indexes identified during review of high-traffic query patterns.
-- Skips indexes that already exist from earlier migrations (063, 073, 085).

-- ck_risks: filter by status (org_id index exists, but no composite with status)
CREATE INDEX IF NOT EXISTS idx_ck_risks_org_status
  ON ck_risks(org_id, status);

-- ck_capas: filter by status + due_date (overdue queries).
-- idx_ck_capas_org (org_id, status) exists — this adds due_date for range queries.
CREATE INDEX IF NOT EXISTS idx_ck_capas_org_status_due
  ON ck_capas(org_id, status, due_date);

-- ck_controls: filter by framework + status
CREATE INDEX IF NOT EXISTS idx_ck_controls_framework_status
  ON ck_controls(framework_id, manual_status);

-- ck_incidents: filter by org + created_at (recent incidents).
-- idx_ck_incidents_org_id (org_id) exists — this adds created_at for date ordering.
CREATE INDEX IF NOT EXISTS idx_ck_incidents_org_created
  ON ck_incidents(org_id, created_at DESC);

-- audit_log: audit_log_org_idx (org_id, created_at DESC) already exists from 085 — skip.

-- hr_employees: hr_employees_org_idx (org_id, status) already exists from 063 — skip.

-- po_dsr: filter by org + status + due_date for overdue queries.
-- idx_po_dsr_org (org_id, status, received_at DESC) exists — this is a different composite for due_date.
CREATE INDEX IF NOT EXISTS idx_po_dsr_org_status_due
  ON po_dsr(org_id, status, due_date)
  WHERE status NOT IN ('completed', 'rejected');
