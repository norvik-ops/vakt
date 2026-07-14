-- Zurueck auf den CHECK ohne 'sent'.
--
-- Die sent-Events MUESSEN dabei geloescht werden — der alte CHECK laesst sie nicht zu,
-- und ein ADD CONSTRAINT gegen bestehende Zeilen, die ihn verletzen, schlaegt fehl. Ein
-- Down-Migration, die nicht laeuft, ist schlimmer als eine, die etwas wegnimmt.
--
-- Was dabei verloren geht, ist die Aufloesbarkeit der Tracking-Token: Nach diesem
-- Rollback misst Vakt Aware wieder nichts (siehe up-Migration). Die open/click-Events
-- selbst bleiben stehen, nur der Sende-Nachweis verschwindet.
DELETE FROM sr_events WHERE type = 'sent';

ALTER TABLE sr_events DROP CONSTRAINT IF EXISTS pg_events_type_check;

ALTER TABLE sr_events ADD CONSTRAINT pg_events_type_check
    CHECK (type IN ('open', 'click', 'form_submission'));
