ALTER TABLE organizations
    DROP COLUMN IF EXISTS trust_center_enabled,
    DROP COLUMN IF EXISTS trust_center_description,
    DROP COLUMN IF EXISTS trust_center_contact;
