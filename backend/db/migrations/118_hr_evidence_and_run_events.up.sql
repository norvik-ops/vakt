-- ck_evidence.control_id nullable machen.
-- Grund: HR-Checklist-Completion-Evidence und zukünftige automatische Evidence-Quellen
-- können nicht vorab einem Control zugeordnet werden — der Compliance-Manager verknüpft
-- sie nachträglich im UI mit den passenden Controls.
ALTER TABLE ck_evidence ALTER COLUMN control_id DROP NOT NULL;

-- Step-Completion Audit-Trail für hr_checklist_runs.
-- completed_items im Run hält nur die IDs der erledigten Schritte (für Schnellabfrage).
-- hr_run_events hält den vollen Audit-Trail: wer hat wann welchen Schritt abgeschlossen.
CREATE TABLE hr_run_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id       UUID NOT NULL REFERENCES hr_checklist_runs(id) ON DELETE CASCADE,
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    step_id      TEXT NOT NULL,
    completed_by TEXT NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX hr_run_events_run_idx ON hr_run_events(run_id);
CREATE INDEX hr_run_events_org_idx ON hr_run_events(org_id);
