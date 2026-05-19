-- Remove MSP multi-tenancy columns from organizations.
-- Vakt is a single-tenant self-hosted product; each customer runs their own instance.
-- The MSP parent-child management layer is not part of the product.
-- RLS policies added in migration 012 are kept as they enforce org isolation.

DROP INDEX IF EXISTS idx_organizations_parent_org_id;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS parent_org_id,
    DROP COLUMN IF EXISTS msp_brand_logo,
    DROP COLUMN IF EXISTS msp_brand_colors,
    DROP COLUMN IF EXISTS scheduled_deletion_at;

-- Keep the `plan` column for potential future licensing use but reset to a neutral default.
ALTER TABLE organizations ALTER COLUMN plan SET DEFAULT 'community';
UPDATE organizations SET plan = 'community' WHERE plan IN ('msp_managed', 'msp_parent');
