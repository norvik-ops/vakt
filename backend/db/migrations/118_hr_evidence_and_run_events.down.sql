DROP TABLE IF EXISTS hr_run_events;

-- ck_evidence.control_id wieder NOT NULL setzen ist nur möglich, wenn keine NULL-Werte existieren.
-- Im Rollback-Fall werden alle bestehenden Evidence-Einträge ohne Control-Zuordnung gelöscht.
DELETE FROM ck_evidence WHERE control_id IS NULL;
ALTER TABLE ck_evidence ALTER COLUMN control_id SET NOT NULL;
