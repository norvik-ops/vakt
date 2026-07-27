-- Restore the pre-249 column default. The backfill (UPDATE) is data, not schema,
-- and is intentionally NOT reverted: we cannot distinguish rows that were silently
-- defaulted to 'admin' from rows an admin explicitly set to 'admin', so re-inflating
-- them would re-introduce exactly the escalation this migration removed. Down only
-- restores the schema default.
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'admin';
