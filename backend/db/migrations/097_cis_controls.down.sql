-- Migration 097 down: remove all CIS Controls v8 data
-- Removes controls and frameworks seeded for CIS (all orgs).

DELETE FROM ck_controls
WHERE framework_id IN (
    SELECT id FROM ck_frameworks WHERE name = 'CIS' AND is_builtin = true
);

DELETE FROM ck_frameworks WHERE name = 'CIS' AND is_builtin = true;
