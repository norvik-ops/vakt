-- S70-4: Contractor/Freelancer Lifecycle
CREATE TABLE hr_contractors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT,
    company TEXT,
    contract_start DATE NOT NULL,
    contract_end DATE NOT NULL,
    access_scope TEXT[] NOT NULL DEFAULT '{}',
    nda_signed BOOLEAN NOT NULL DEFAULT false,
    avv_signed BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL CHECK (status IN ('active', 'expiring_soon', 'offboarding', 'terminated')) DEFAULT 'active',
    checklist_run_id UUID,
    offboarding_completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_hr_contractors_org ON hr_contractors (org_id, status);
CREATE INDEX idx_hr_contractors_end ON hr_contractors (contract_end) WHERE status IN ('active', 'expiring_soon');

-- S70-5: Vault Access Review (quartalsweise)
CREATE TABLE so_access_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_label TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('open', 'completed')) DEFAULT 'open',
    reviewed_by UUID REFERENCES users(id),
    completed_at TIMESTAMPTZ,
    total_entries INTEGER NOT NULL DEFAULT 0,
    stale_entries INTEGER NOT NULL DEFAULT 0,
    revoked_entries INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_so_access_reviews_org ON so_access_reviews (org_id, status);
