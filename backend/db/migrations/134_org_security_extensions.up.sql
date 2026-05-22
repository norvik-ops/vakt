-- S21-5: Per-org admin IP allowlist (Pro).
-- S21-6: Per-org MFA requirement for sensitive API calls (Pro).
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS admin_ip_allowlist         TEXT NULL,      -- comma-separated CIDRs; NULL = allow all
    ADD COLUMN IF NOT EXISTS require_mfa_sensitive_calls BOOLEAN NOT NULL DEFAULT false;
