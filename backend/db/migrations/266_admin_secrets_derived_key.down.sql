-- Revert the format documentation from migration 266.
--
-- The DATA is not reverted here, for the same reason it is not migrated here:
-- SQL cannot decrypt. Rolling the ciphertext back to the raw-master form is
--
--     rotate-key admin-rekey down
--
-- and it must be run BEFORE deploying a release older than migration 266 —
-- that release only knows the legacy format. Dropping these comments without
-- running it leaves the data in the derived-key form, which the old code
-- cannot open; the data is not lost, but the rollback is not complete either.

COMMENT ON COLUMN org_oidc_configs.client_secret_enc IS NULL;
COMMENT ON COLUMN organizations.smtp_pass_enc IS NULL;
COMMENT ON COLUMN organizations.ldap_bind_pass_enc IS NULL;
COMMENT ON COLUMN organizations.backup_passphrase_enc IS NULL;
COMMENT ON COLUMN organizations.backup_notify_webhook_enc IS NULL;
COMMENT ON COLUMN organizations.backup_dest_config_enc IS NULL;
