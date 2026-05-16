-- Reverse audit log migration
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_timestamp;
DROP INDEX IF EXISTS idx_audit_logs_org_id;

DROP TABLE IF EXISTS audit_logs;
