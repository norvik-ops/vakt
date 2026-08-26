-- Die entdoppelten Zeilen kommen nicht zurueck; das ist bei einem
-- Duplikat-Rueckbau unvermeidlich und beabsichtigt.
DROP INDEX IF EXISTS ck_control_measures_builtin_uniq;
DROP INDEX IF EXISTS ck_evidence_collector_uniq;
