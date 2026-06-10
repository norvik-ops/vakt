-- S67-3: Informationsklassifizierung (ISO 27001 A.5.12-13)
-- Adds classification label to assets and policies.

ALTER TABLE vb_assets
    ADD COLUMN IF NOT EXISTS classification TEXT
        CHECK (classification IN ('public', 'internal', 'confidential', 'restricted'))
        DEFAULT 'internal';

ALTER TABLE ck_policies
    ADD COLUMN IF NOT EXISTS classification TEXT
        CHECK (classification IN ('public', 'internal', 'confidential', 'restricted'))
        DEFAULT 'internal';

CREATE INDEX IF NOT EXISTS idx_vb_assets_classification ON vb_assets (org_id, classification);
CREATE INDEX IF NOT EXISTS idx_ck_policies_classification ON ck_policies (org_id, classification);
