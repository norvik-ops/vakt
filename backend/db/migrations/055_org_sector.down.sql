ALTER TABLE organizations
    DROP COLUMN IF EXISTS sector,
    DROP COLUMN IF EXISTS federal_state;
