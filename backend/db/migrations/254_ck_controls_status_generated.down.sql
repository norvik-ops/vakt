-- Zurueck auf den Zustand von Migration 153: eine gewoehnliche Spalte mit
-- Vorgabewert 'missing', aus dem not_applicable-Flag vorbelegt.
DROP INDEX IF EXISTS idx_ck_controls_org_status;

ALTER TABLE ck_controls DROP COLUMN IF EXISTS status;

ALTER TABLE ck_controls
  ADD COLUMN status TEXT NOT NULL DEFAULT 'missing';

UPDATE ck_controls SET status = 'not_applicable' WHERE not_applicable = true;
