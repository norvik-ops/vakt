DROP INDEX IF EXISTS idx_audit_log_siem_pending;

ALTER TABLE audit_log
    DROP COLUMN IF EXISTS forwarded_to_siem;
