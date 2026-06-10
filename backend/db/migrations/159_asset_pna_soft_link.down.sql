ALTER TABLE vb_assets DROP COLUMN IF EXISTS protection_need_id;
ALTER TABLE ck_protection_need_assessments DROP COLUMN IF EXISTS vb_asset_id;
