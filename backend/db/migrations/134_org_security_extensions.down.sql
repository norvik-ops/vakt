ALTER TABLE organizations
    DROP COLUMN IF EXISTS admin_ip_allowlist,
    DROP COLUMN IF EXISTS require_mfa_sensitive_calls;
