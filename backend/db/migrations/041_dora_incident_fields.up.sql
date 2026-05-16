-- 041: Add DORA-specific fields to ck_incidents
-- These fields support Art. 18 DORA major incident reporting requirements.

ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS affected_customers        INT,
    ADD COLUMN IF NOT EXISTS financial_impact_estimate TEXT,
    ADD COLUMN IF NOT EXISTS is_major_incident         BOOLEAN NOT NULL DEFAULT false;
