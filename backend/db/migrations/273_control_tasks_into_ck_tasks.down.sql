-- Rückweg zu Migration 273.
--
-- 1. Den Stand der übernommenen Aufgaben (Titel, erledigt, updated_at) in die
--    alte Tabelle zurückschreiben. Wer nach dem Upgrade abgehakt hat, verliert
--    das beim Downgrade nicht.
-- 2. Genau die übernommenen Zeilen aus ck_tasks löschen, erkennbar an
--    source_control_task_id. Aufgaben, die nach dem Upgrade neu angelegt wurden,
--    bleiben in ck_tasks und erscheinen in der alten Version unter
--    „Zugewiesene Aufgaben“.
-- Grenze: Eine übernommene Aufgabe, die nach dem Upgrade gelöscht wurde, steht
-- nach dem Downgrade wieder in der alten Checkliste (die alte Zeile wurde nie
-- gelöscht).

UPDATE ck_control_tasks ct
SET text       = t.title,
    completed  = (t.status = 'done'),
    updated_at = t.updated_at
FROM ck_tasks t
WHERE t.source_control_task_id = ct.id;

DELETE FROM ck_tasks WHERE source_control_task_id IS NOT NULL;

DROP INDEX IF EXISTS idx_ck_tasks_source_control_task;

ALTER TABLE ck_tasks DROP COLUMN IF EXISTS source_control_task_id;

COMMENT ON TABLE ck_control_tasks IS NULL;
