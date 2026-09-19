-- R1-36b-SC06: recurring scan schedules (vb_scan_schedules) had no executor and
-- therefore never ran. The executor (cmd/worker/schedule_executor_scan.go) selects
-- due schedules (active rows whose next_run is unset or already elapsed at query time)
-- and, after enqueuing, advances next_run from each schedule's cron expression.
--
-- No new columns are added: vb_scan_schedules already carries next_run/last_run
-- (TIMESTAMPTZ, migration 007) with exactly the semantics this fix needs — the
-- executor reuses them. Adding a second next_run_at/last_run_at pair would
-- duplicate them and drift from the sqlc query that already returns next_run/last_run.
--
-- This migration only prepares those existing columns for the executor:
--   1. Backfill next_run = NOW() for active schedules that never had one set,
--      so the first executor tick picks them up and then writes the correct
--      cron-derived next_run.
--   2. Add a partial index on (next_run) WHERE is_active so the due-scan query
--      is an index range scan, not a sequential scan.

-- 1. Backfill: give existing active schedules an immediate first run.
UPDATE vb_scan_schedules
SET next_run = NOW()
WHERE next_run IS NULL
  AND is_active = TRUE;

-- 2. Partial index for the due-scan query. WHERE uses only the plain boolean
--    column is_active — no volatile function in the predicate, so this is
--    SQLSTATE 42P17-safe. Plain (non-CONCURRENT) CREATE INDEX runs inside the
--    migration transaction.
CREATE INDEX IF NOT EXISTS idx_vb_scan_schedules_due
    ON vb_scan_schedules (next_run)
    WHERE is_active;
