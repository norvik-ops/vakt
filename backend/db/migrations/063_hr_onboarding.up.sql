CREATE TABLE IF NOT EXISTS hr_employees (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    first_name  TEXT NOT NULL,
    last_name   TEXT NOT NULL,
    email       TEXT NOT NULL,
    department  TEXT,
    role        TEXT,
    start_date  DATE,
    end_date    DATE,
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'offboarding', 'terminated')),
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, email)
);
CREATE INDEX IF NOT EXISTS hr_employees_org_idx ON hr_employees(org_id, status);

CREATE TABLE IF NOT EXISTS hr_checklists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('onboarding', 'offboarding')),
    name        TEXT NOT NULL,
    items       JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS hr_checklist_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    employee_id     UUID NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    checklist_id    UUID NOT NULL REFERENCES hr_checklists(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed')),
    completed_items JSONB NOT NULL DEFAULT '[]',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS hr_checklist_runs_employee_idx ON hr_checklist_runs(employee_id);
