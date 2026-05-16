-- Training module library
CREATE TABLE pg_training_modules (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('video','quiz')),
    attack_type      TEXT NOT NULL CHECK (attack_type IN ('phishing','vishing','usb','smishing')),
    content_url      TEXT NOT NULL,
    duration_seconds INT NOT NULL DEFAULT 0,
    passing_score    INT NOT NULL DEFAULT 80,
    questions        JSONB,
    created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Training assignments (employee to module)
CREATE TABLE pg_assignments (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    module_id     UUID NOT NULL REFERENCES pg_training_modules(id) ON DELETE CASCADE,
    target_id     UUID REFERENCES pg_targets(id) ON DELETE SET NULL,
    department    TEXT,
    due_date      TIMESTAMPTZ NOT NULL,
    is_overdue    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(module_id, target_id) DEFERRABLE INITIALLY DEFERRED
);

-- Completion records
CREATE TABLE pg_completions (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    assignment_id UUID NOT NULL REFERENCES pg_assignments(id) ON DELETE CASCADE UNIQUE,
    score        INT,
    passed       BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pg_assignments_target_id ON pg_assignments(target_id) WHERE target_id IS NOT NULL;
CREATE INDEX idx_pg_assignments_module_id ON pg_assignments(module_id);
