-- Migration 154 rollback: restore original check constraint (without vakthr)
-- Note: any existing vakthr rows must be deleted first or the constraint add will fail.

DELETE FROM user_module_permissions WHERE module = 'vakthr';

ALTER TABLE user_module_permissions
    DROP CONSTRAINT IF EXISTS user_module_permissions_module_check;

ALTER TABLE user_module_permissions
    ADD CONSTRAINT user_module_permissions_module_check
    CHECK (module IN ('vaktscan', 'vaktcomply', 'vaktvault', 'vaktaware', 'vaktprivacy'));
