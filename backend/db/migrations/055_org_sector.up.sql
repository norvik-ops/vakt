-- 055: Sector and federal state fields for NIS2 authority mapping (Story 31.4)
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS sector        TEXT NOT NULL DEFAULT 'other',
    ADD COLUMN IF NOT EXISTS federal_state TEXT;
