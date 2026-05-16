-- 041 rollback: Remove DORA-specific fields from ck_incidents

ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS affected_customers,
    DROP COLUMN IF EXISTS financial_impact_estimate,
    DROP COLUMN IF EXISTS is_major_incident;
