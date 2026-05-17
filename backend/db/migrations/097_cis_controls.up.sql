-- Migration 097: CIS Controls v8 framework registration
-- CIS Controls v8 has 18 control groups (IG1/IG2/IG3 implementation groups)
-- and is available as a builtin framework in Vakt Comply.
--
-- Per-org framework rows and controls are seeded by the Go service at startup
-- (builtinControls() in service.go) whenever an org activates the "CIS" framework.
-- This migration serves as a version marker and removes any stale CIS controls
-- from a previous partial seed so the startup seeder can recreate them cleanly.

DELETE FROM ck_controls
WHERE framework_id IN (
    SELECT id FROM ck_frameworks WHERE name = 'CIS' AND is_builtin = true
);
