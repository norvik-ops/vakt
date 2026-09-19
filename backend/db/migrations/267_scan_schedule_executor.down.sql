-- Revert R1-36b-SC06 executor preparation.
DROP INDEX IF EXISTS idx_vb_scan_schedules_due;

-- The next_run backfill is intentionally not reverted: NULL vs NOW() carries no
-- information worth restoring, the next_run/last_run columns pre-date this
-- migration (007), and un-setting next_run would only re-trigger a first run.
