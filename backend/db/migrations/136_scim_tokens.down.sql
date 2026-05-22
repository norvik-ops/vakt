-- Rollback migration 136

DROP TABLE IF EXISTS scim_group_members;
DROP TABLE IF EXISTS scim_groups;

DROP INDEX IF EXISTS idx_users_scim_external_id;
ALTER TABLE users
    DROP COLUMN IF EXISTS scim_provisioned,
    DROP COLUMN IF EXISTS scim_external_id;

DROP INDEX IF EXISTS idx_scim_tokens_token_hash;
DROP INDEX IF EXISTS idx_scim_tokens_org_id;
DROP TABLE IF EXISTS scim_tokens;
