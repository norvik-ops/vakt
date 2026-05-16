-- Reverse SecretOps schema migration

DROP INDEX IF EXISTS idx_so_share_links_token_hash;
DROP INDEX IF EXISTS idx_so_access_log_secret_id;
DROP INDEX IF EXISTS idx_so_secrets_org_id;
DROP INDEX IF EXISTS idx_so_secrets_env_id;

DROP TABLE IF EXISTS so_master_key_fingerprint;
DROP TABLE IF EXISTS so_share_links;
DROP TABLE IF EXISTS so_access_log;
DROP TABLE IF EXISTS so_secrets;
DROP TABLE IF EXISTS so_environments;
DROP TABLE IF EXISTS so_projects;
