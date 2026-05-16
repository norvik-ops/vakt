DROP TABLE IF EXISTS ck_control_reviews;
ALTER TABLE ck_controls
  DROP COLUMN IF EXISTS last_reviewed_at,
  DROP COLUMN IF EXISTS review_interval_days,
  DROP COLUMN IF EXISTS next_review_due,
  DROP COLUMN IF EXISTS last_reviewed_by,
  DROP COLUMN IF EXISTS review_note;
