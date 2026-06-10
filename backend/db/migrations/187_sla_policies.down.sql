DROP INDEX IF EXISTS idx_vb_findings_sla;
ALTER TABLE vb_findings
    DROP COLUMN IF EXISTS sla_due_at,
    DROP COLUMN IF EXISTS sla_status,
    DROP COLUMN IF EXISTS sla_resolved_within,
    DROP COLUMN IF EXISTS sla_actual_days;
DROP TABLE IF EXISTS vb_sla_policies;
