-- 154_audit_log_soft_delete.down.sql
DROP INDEX IF EXISTS idx_audit_log_deleted_at;
ALTER TABLE audit_log DROP COLUMN IF EXISTS deleted_at;
