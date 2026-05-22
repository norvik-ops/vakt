-- Rollback S39 BSI reporting columns.
ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS classification_result,
    DROP COLUMN IF EXISTS reporting_deadlines;
