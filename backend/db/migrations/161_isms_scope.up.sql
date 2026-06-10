CREATE TABLE ck_isms_scope (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL CHECK (status IN ('draft', 'approved')) DEFAULT 'draft',
    scope_definition TEXT NOT NULL DEFAULT '',
    exclusions JSONB NOT NULL DEFAULT '[]',
    outsourcing_dependencies TEXT NOT NULL DEFAULT '',
    change_note TEXT NOT NULL DEFAULT '',
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ck_isms_scope_org ON ck_isms_scope (org_id, version DESC);
