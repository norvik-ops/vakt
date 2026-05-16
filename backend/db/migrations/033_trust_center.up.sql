ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS trust_center_enabled     BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS trust_center_description TEXT,
    ADD COLUMN IF NOT EXISTS trust_center_contact     TEXT;
