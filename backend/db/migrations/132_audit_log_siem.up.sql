ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS forwarded_to_siem TIMESTAMPTZ NULL;
CREATE INDEX IF NOT EXISTS idx_audit_log_siem_pending
    ON audit_log (created_at) WHERE forwarded_to_siem IS NULL;
