-- Restore MSP columns (rollback only — MSP feature has been removed from Vakt)
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS parent_org_id UUID REFERENCES organizations(id),
    ADD COLUMN IF NOT EXISTS msp_brand_logo TEXT,
    ADD COLUMN IF NOT EXISTS msp_brand_colors JSONB DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS scheduled_deletion_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_organizations_parent_org_id ON organizations(parent_org_id)
    WHERE parent_org_id IS NOT NULL;

ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'standard';
