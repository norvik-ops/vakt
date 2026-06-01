-- Add status column to ck_controls.
-- The original migration (006) did not include status; code references
-- c.status with values 'not_applicable', 'missing', 'compliant', etc.
-- Seed from the existing not_applicable boolean; all other controls default to 'missing'.

ALTER TABLE ck_controls
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'missing';

UPDATE ck_controls SET status = 'not_applicable' WHERE not_applicable = true;
