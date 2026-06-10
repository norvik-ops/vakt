-- S69-4: JML Mover Workflow (Joiner-Mover-Leaver)
-- Tracks role/department changes and triggers access revoke + grant checklists.

CREATE TABLE IF NOT EXISTS hr_mover_events (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id         UUID        NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    from_department     TEXT,
    from_job_title      TEXT,
    to_department       TEXT        NOT NULL,
    to_job_title        TEXT        NOT NULL,
    effective_date      DATE        NOT NULL,
    initiated_by        UUID        REFERENCES users(id),
    checklist_run_id    UUID,
    status              TEXT        NOT NULL CHECK (status IN (
        'pending',
        'in_progress',
        'completed',
        'overdue',
        'cancelled'
    )) DEFAULT 'pending',
    due_date            DATE        NOT NULL,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hr_mover_events_org      ON hr_mover_events (org_id, status);
CREATE INDEX IF NOT EXISTS idx_hr_mover_events_employee ON hr_mover_events (employee_id);
CREATE INDEX IF NOT EXISTS idx_hr_mover_events_due      ON hr_mover_events (due_date)
    WHERE status NOT IN ('completed');

CREATE TABLE IF NOT EXISTS hr_mover_templates (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL,
    from_role_hint  TEXT,
    to_role_hint    TEXT,
    is_default      BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS hr_mover_template_items (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID        NOT NULL REFERENCES hr_mover_templates(id) ON DELETE CASCADE,
    section         TEXT        NOT NULL CHECK (section IN ('revoke', 'grant', 'verify')),
    title           TEXT        NOT NULL,
    description     TEXT,
    responsible_role TEXT       CHECK (responsible_role IN ('hr', 'it', 'manager', 'employee')),
    sort_order      INTEGER     NOT NULL DEFAULT 0
);
