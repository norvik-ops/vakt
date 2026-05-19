-- Policy templates: pre-built compliance document templates for policies, DPIAs, and AVVs.
CREATE TABLE ck_policy_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category    TEXT NOT NULL CHECK (category IN ('policy', 'dpia', 'avv')),
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    content     TEXT NOT NULL,
    tags        TEXT[] NOT NULL DEFAULT '{}',
    framework   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_policy_templates_category ON ck_policy_templates(category);
