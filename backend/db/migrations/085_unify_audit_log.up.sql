-- Migrate audit data: consolidate the two audit tables into audit_log.
-- The older audit_logs table (migration 004) stores HTTP-middleware events
-- with status_code, user_agent, and timestamp. The newer audit_log table
-- (migration 064) stores compliance-domain events with user_email,
-- resource_name, details, and created_at.
--
-- Strategy:
-- 1. Copy rows from audit_logs into audit_log (mapping columns best-effort).
-- 2. Drop the audit_logs table and replace it with a VIEW over audit_log so
--    that any remaining code paths that query audit_logs continue to work.
--    The view exposes a timestamp column (aliased from created_at) and returns
--    NULL for columns that no longer exist (user_agent, status_code).

-- Step 1: Migrate historical data from audit_logs → audit_log.
INSERT INTO audit_log
    (org_id, user_id, action, resource_type, resource_id, ip_address, created_at)
SELECT
    org_id,
    user_id,
    action,
    resource_type,
    resource_id,
    ip_address,
    timestamp
FROM audit_logs
ON CONFLICT DO NOTHING;

-- Step 2: Drop the now-redundant audit_logs table.
DROP TABLE audit_logs;

-- Step 3: Create a backward-compatible view so that any remaining references
-- to audit_logs (e.g. admin service, retention service) continue to compile
-- and run without errors.  status_code and user_agent are exposed as NULLs.
CREATE VIEW audit_logs AS
SELECT
    id,
    org_id,
    user_id,
    action,
    resource_type,
    resource_id,
    ip_address,
    NULL::text    AS user_agent,
    NULL::int     AS status_code,
    created_at    AS timestamp
FROM audit_log;
