ALTER TABLE ck_bcp_plans
    DROP COLUMN IF EXISTS rto_hours,
    DROP COLUMN IF EXISTS rpo_hours,
    DROP COLUMN IF EXISTS schutzbedarfsklasse,
    DROP COLUMN IF EXISTS last_tested_at;

DROP TABLE IF EXISTS ck_bia_processes;
