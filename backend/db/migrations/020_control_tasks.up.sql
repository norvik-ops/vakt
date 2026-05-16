-- Implementation task checklist items for compliance controls.
CREATE TABLE IF NOT EXISTS ck_control_tasks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    control_id UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    completed  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ck_control_tasks_control ON ck_control_tasks(control_id, org_id);
