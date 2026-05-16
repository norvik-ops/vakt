-- Revert E16: MSP Multi-tenancy

-- Drop RLS policies
DROP POLICY IF EXISTS pg_campaigns_org ON pg_campaigns;
DROP POLICY IF EXISTS so_secrets_org ON so_secrets;
DROP POLICY IF EXISTS so_projects_org ON so_projects;
DROP POLICY IF EXISTS ck_evidence_org ON ck_evidence;
DROP POLICY IF EXISTS ck_controls_org ON ck_controls;
DROP POLICY IF EXISTS ck_frameworks_org ON ck_frameworks;
DROP POLICY IF EXISTS vb_findings_org ON vb_findings;
DROP POLICY IF EXISTS vb_assets_org ON vb_assets;

-- Disable RLS on module tables
ALTER TABLE pg_campaigns DISABLE ROW LEVEL SECURITY;
ALTER TABLE so_secrets DISABLE ROW LEVEL SECURITY;
ALTER TABLE so_projects DISABLE ROW LEVEL SECURITY;
ALTER TABLE ck_evidence DISABLE ROW LEVEL SECURITY;
ALTER TABLE ck_controls DISABLE ROW LEVEL SECURITY;
ALTER TABLE ck_frameworks DISABLE ROW LEVEL SECURITY;
ALTER TABLE vb_findings DISABLE ROW LEVEL SECURITY;
ALTER TABLE vb_assets DISABLE ROW LEVEL SECURITY;

-- Drop parent org index
DROP INDEX IF EXISTS idx_organizations_parent_org_id;

-- Remove MSP columns from organizations
ALTER TABLE organizations DROP COLUMN IF EXISTS scheduled_deletion_at;
ALTER TABLE organizations DROP COLUMN IF EXISTS msp_brand_colors;
ALTER TABLE organizations DROP COLUMN IF EXISTS msp_brand_logo;
ALTER TABLE organizations DROP COLUMN IF EXISTS plan;
ALTER TABLE organizations DROP COLUMN IF EXISTS parent_org_id;
