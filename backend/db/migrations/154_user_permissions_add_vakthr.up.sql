-- Migration 154: add vakthr to user_module_permissions check constraint
-- The HR module (vakthr) was omitted from the original module list in migration 086.
-- This migration drops the old check constraint and recreates it with all 6 modules.

ALTER TABLE user_module_permissions
    DROP CONSTRAINT IF EXISTS user_module_permissions_module_check;

ALTER TABLE user_module_permissions
    ADD CONSTRAINT user_module_permissions_module_check
    CHECK (module IN ('vaktscan', 'vaktcomply', 'vaktvault', 'vaktaware', 'vaktprivacy', 'vakthr'));
