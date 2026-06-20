-- Revert key_pem to TEXT. hex-encode so the conversion never fails on binary
-- data (the app would not read this form — the down is for schema rollback only).
ALTER TABLE org_saml_configs
    ALTER COLUMN key_pem TYPE TEXT USING encode(key_pem, 'hex');
