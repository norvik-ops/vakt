-- S44-2: Add environment field to vb_assets (prod/staging/dev).
ALTER TABLE vb_assets
    ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'prod'
        CHECK (environment IN ('prod', 'staging', 'dev'));

CREATE INDEX IF NOT EXISTS idx_vb_assets_environment ON vb_assets (org_id, environment);
