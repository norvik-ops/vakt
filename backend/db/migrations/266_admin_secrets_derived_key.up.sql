-- Admin-managed secrets move from the RAW master key onto the HKDF sub-key
-- "vakt-admin-v1", marked with the prefix 'enc:adm1:'.
--
-- Until now these six columns were the only encrypted columns in the product
-- still sealed with the raw master key, and cmd/rotate-key had no stage for any
-- of them: a master-key swap made the SSO client secret, the SMTP and LDAP
-- credentials and the backup passphrase PERMANENTLY undecryptable, silently.
--
-- WHY THERE IS NO DATA STATEMENT HERE, and where the data step actually lives:
-- re-encrypting means AES-256-GCM plus HKDF-SHA256. golang-migrate executes
-- plain SQL, and this database has neither (pgcrypto offers no AES-GCM and no
-- HKDF, and is not installed). The re-key is therefore a Go step, in the tool
-- that already owns every other at-rest key operation:
--
--     rotate-key admin-rekey up      -- legacy raw master -> derived sub-key
--     rotate-key admin-rekey down    -- derived sub-key   -> legacy raw master
--
-- It is idempotent (the marker tells each value's format, so a second run
-- converts nothing twice), it ABORTS and names the row if a value opens under
-- neither format, and `down` restores the byte-for-byte original storage form
-- so the previous release can read the database again.
--
-- Running it is not required for correctness: internal/admin reads both forms
-- (openSecret), and each rotation stage migrates a legacy value on the way
-- past. It is what bounds how long raw-master ciphertext survives on disk.
--
-- This migration records the format at the place an engineer actually looks
-- when they meet one of these columns — \d+ organizations. Deliberately NOT a
-- version column or state table: the marker on each value is the single source
-- of truth for its format, and a second one could only drift away from it.

COMMENT ON COLUMN org_oidc_configs.client_secret_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): ''enc:v1:'' + base64(raw-master ciphertext). Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';

COMMENT ON COLUMN organizations.smtp_pass_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): bare raw-master ciphertext. Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';

COMMENT ON COLUMN organizations.ldap_bind_pass_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): bare raw-master ciphertext. Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';

COMMENT ON COLUMN organizations.backup_passphrase_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): bare raw-master ciphertext. Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';

COMMENT ON COLUMN organizations.backup_notify_webhook_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): bare raw-master ciphertext. Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';

COMMENT ON COLUMN organizations.backup_dest_config_enc IS
  'AES-256-GCM. Migration 266: sealed with HKDF sub-key "vakt-admin-v1" and prefixed ''enc:adm1:''. Legacy form (pre-266): bare raw-master ciphertext. Rotated by cmd/rotate-key; converted by `rotate-key admin-rekey up|down`.';
