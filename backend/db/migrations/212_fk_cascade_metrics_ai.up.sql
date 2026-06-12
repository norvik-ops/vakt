-- 212_fk_cascade_metrics_ai.up.sql
-- Add ON DELETE CASCADE to org_id columns in AI and metrics tables.

-- ai_pending_approvals
DELETE FROM ai_pending_approvals WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE ai_pending_approvals
    ADD CONSTRAINT fk_ai_pending_approvals_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- vb_risk_trend_snapshots
DELETE FROM vb_risk_trend_snapshots WHERE org_id NOT IN (SELECT id FROM organizations);
ALTER TABLE vb_risk_trend_snapshots
    ADD CONSTRAINT fk_vb_risk_trend_snapshots_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE;
