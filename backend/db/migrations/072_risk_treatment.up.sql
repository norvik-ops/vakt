-- Migration 071: Risk treatment workflow + risk-to-control linking
-- Adds treatment plan, residual risk scoring, and a dedicated risk↔control link table.

-- Add treatment fields to existing ck_risks table
ALTER TABLE ck_risks
  ADD COLUMN IF NOT EXISTS treatment_option TEXT DEFAULT '' CHECK (treatment_option IN ('','accept','mitigate','transfer','avoid')),
  ADD COLUMN IF NOT EXISTS treatment_plan TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS treatment_owner TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS treatment_due_date DATE,
  ADD COLUMN IF NOT EXISTS treatment_status TEXT NOT NULL DEFAULT 'pending' CHECK (treatment_status IN ('pending','in_progress','implemented','verified')),
  ADD COLUMN IF NOT EXISTS residual_likelihood INT CHECK (residual_likelihood BETWEEN 1 AND 5),
  ADD COLUMN IF NOT EXISTS residual_impact INT CHECK (residual_impact BETWEEN 1 AND 5);

-- Risk ↔ Control links (may already exist from an earlier migration; use IF NOT EXISTS)
CREATE TABLE IF NOT EXISTS ck_risk_control_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  risk_id UUID NOT NULL REFERENCES ck_risks(id) ON DELETE CASCADE,
  control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, risk_id, control_id)
);
CREATE INDEX IF NOT EXISTS idx_ck_rcl_risk ON ck_risk_control_links(org_id, risk_id);
CREATE INDEX IF NOT EXISTS idx_ck_rcl_control ON ck_risk_control_links(org_id, control_id);
