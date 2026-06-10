-- Migration 159: Soft-link between vb_assets (vaktscan) and ck_protection_need_assessments (vaktcomply).
-- No FK constraints — module isolation is maintained via org_id scope only.
-- A NULL value means "not linked". Both columns are optional.

ALTER TABLE ck_protection_need_assessments
    ADD COLUMN IF NOT EXISTS vb_asset_id UUID;

ALTER TABLE vb_assets
    ADD COLUMN IF NOT EXISTS protection_need_id UUID;
