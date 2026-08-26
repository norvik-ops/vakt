-- Kampagnen im neuen Zustand muessen vor dem Zurueckrollen der Bedingung
-- weichen, sonst schlaegt der ALTER an genau den Zeilen fehl, die der Fix
-- ueberhaupt erst sichtbar gemacht hat.
UPDATE sr_campaigns SET status = 'aborted' WHERE status = 'failed';

ALTER TABLE sr_campaigns DROP CONSTRAINT IF EXISTS pg_campaigns_status_check;

ALTER TABLE sr_campaigns ADD CONSTRAINT pg_campaigns_status_check
    CHECK (status IN ('draft', 'scheduled', 'running', 'completed', 'aborted'));
