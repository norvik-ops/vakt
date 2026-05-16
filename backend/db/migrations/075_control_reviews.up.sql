-- Add review tracking columns to existing ck_controls table
ALTER TABLE ck_controls
  ADD COLUMN IF NOT EXISTS last_reviewed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS review_interval_days INT NOT NULL DEFAULT 365,
  ADD COLUMN IF NOT EXISTS next_review_due TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_reviewed_by TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS review_note TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_ck_controls_review_due
  ON ck_controls(org_id, next_review_due)
  WHERE next_review_due IS NOT NULL;

-- Review history log
CREATE TABLE IF NOT EXISTS ck_control_reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
  reviewed_by TEXT NOT NULL DEFAULT '',
  review_note TEXT NOT NULL DEFAULT '',
  status_at_review TEXT NOT NULL DEFAULT '',
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ck_cr_control ON ck_control_reviews(control_id, reviewed_at DESC);
