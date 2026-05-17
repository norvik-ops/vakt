-- Rollback migration 086: drop per-user module permission table

DROP INDEX IF EXISTS idx_user_module_perms_user;
DROP TABLE IF EXISTS user_module_permissions;
