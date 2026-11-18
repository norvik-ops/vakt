-- Eine Aufgabenliste je Kontrolle.
--
-- Bisher gab es auf der Kontrollseite zwei Datenmodelle für Aufgaben:
-- ck_control_tasks (Checkliste „Umsetzungsschritte“: Text + erledigt) und
-- ck_tasks (zugewiesene Aufgaben mit Person, Frist, Priorität; hängt auch an
-- Risiken, Vorfällen, Richtlinien und Audits). Ab jetzt ist ck_tasks das einzige
-- Modell. Diese Migration übernimmt jede Zeile aus ck_control_tasks nach ck_tasks:
--
--   text        → title
--   completed   → status ('done' / 'open')
--   control_id  → entity_type = 'control', entity_id = control_id
--   org_id, created_at, updated_at unverändert. created_at trägt die
--   Reihenfolge; updated_at ist der Zeitpunkt des letzten Abhakens (ein eigenes
--   Erledigt-Datum hatte die alte Tabelle nicht).
--   Person und Frist bleiben leer, Priorität 'medium' (Spalten-Default).
--
-- Herkunft: source_control_task_id hält die ID der alten Zeile. Daran erkennt
-- die Down-Migration genau die übernommenen Zeilen, und ein zweiter Lauf
-- übernimmt nichts doppelt (eindeutiger Index + NOT EXISTS).
--
-- ck_control_tasks bleibt bestehen (Rückweg, Bestandsschutz), wird aber nicht
-- mehr geschrieben oder gelesen.
--
-- Kennzahlen: Alle Zähler über ck_tasks (Dashboard „überfällige Aufgaben“,
-- KI-Hinweise, Wochenüberblick, Erinnerung an Kontrollverantwortliche) filtern
-- auf eine gesetzte Frist. Übernommene Zeilen haben keine Frist und ändern
-- diese Zahlen deshalb nicht.

ALTER TABLE ck_tasks
    ADD COLUMN IF NOT EXISTS source_control_task_id UUID NULL;

COMMENT ON COLUMN ck_tasks.source_control_task_id IS
    'Migration 273: ID der übernommenen Zeile aus ck_control_tasks; NULL für alle anderen Aufgaben.';

CREATE UNIQUE INDEX IF NOT EXISTS idx_ck_tasks_source_control_task
    ON ck_tasks (source_control_task_id)
    WHERE source_control_task_id IS NOT NULL;

INSERT INTO ck_tasks
    (org_id, entity_type, entity_id, title, status, created_at, updated_at, source_control_task_id)
SELECT ct.org_id,
       'control',
       ct.control_id,
       ct.text,
       CASE WHEN ct.completed THEN 'done' ELSE 'open' END,
       ct.created_at,
       ct.updated_at,
       ct.id
FROM ck_control_tasks ct
WHERE NOT EXISTS (
    SELECT 1 FROM ck_tasks t WHERE t.source_control_task_id = ct.id
);

COMMENT ON TABLE ck_control_tasks IS
    'DEPRECATED seit Migration 273: Inhalt nach ck_tasks (entity_type = ''control'') übernommen. Wird nicht mehr geschrieben oder gelesen; bleibt für den Rückweg (Down-Migration) bestehen.';
