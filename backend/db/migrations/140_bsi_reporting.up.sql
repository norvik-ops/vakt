-- S39: BSI-Meldungsassistent — classification_result + reporting_deadlines for incidents.
-- classification_result stores the output of the new classify-reporting wizard (S39-1).
-- reporting_deadlines stores computed deadline timestamps as JSONB (S39-2).
ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS classification_result JSONB,
    ADD COLUMN IF NOT EXISTS reporting_deadlines   JSONB;

CREATE INDEX IF NOT EXISTS idx_ck_incidents_classification
    ON ck_incidents ((classification_result->>'obligation'))
    WHERE classification_result IS NOT NULL;
