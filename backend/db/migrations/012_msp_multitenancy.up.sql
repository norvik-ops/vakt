-- E16: MSP Multi-tenancy
-- Adds parent-child org relationship, plan, branding, and scheduled deletion
-- to organizations table, and enables Row Level Security on all module tables.

-- MSP parent-child org relationship
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS parent_org_id UUID REFERENCES organizations(id);
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'standard';
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS msp_brand_logo TEXT;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS msp_brand_colors JSONB DEFAULT '{}';
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS scheduled_deletion_at TIMESTAMPTZ;

-- Index to look up child orgs of a parent efficiently
CREATE INDEX IF NOT EXISTS idx_organizations_parent_org_id ON organizations(parent_org_id)
    WHERE parent_org_id IS NOT NULL;

-- Enable RLS on all module tables
ALTER TABLE vb_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE vb_findings ENABLE ROW LEVEL SECURITY;
ALTER TABLE ck_frameworks ENABLE ROW LEVEL SECURITY;
ALTER TABLE ck_controls ENABLE ROW LEVEL SECURITY;
ALTER TABLE ck_evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE so_projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE so_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE pg_campaigns ENABLE ROW LEVEL SECURITY;

-- RLS policies (app.current_org_id session variable set by the application)
CREATE POLICY vb_assets_org ON vb_assets USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY vb_findings_org ON vb_findings USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY ck_frameworks_org ON ck_frameworks USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY ck_controls_org ON ck_controls USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY ck_evidence_org ON ck_evidence USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY so_projects_org ON so_projects USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY so_secrets_org ON so_secrets USING (org_id::text = current_setting('app.current_org_id', true));
CREATE POLICY pg_campaigns_org ON pg_campaigns USING (org_id::text = current_setting('app.current_org_id', true));
