DROP INDEX IF EXISTS idx_ck_controls_evidence_status;

ALTER TABLE ck_controls
    DROP COLUMN IF EXISTS evidence_max_age_days,
    DROP COLUMN IF EXISTS evidence_status,
    DROP COLUMN IF EXISTS evidence_last_updated,
    DROP COLUMN IF EXISTS evidence_expires_at;
