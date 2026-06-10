DROP INDEX IF EXISTS idx_ck_policies_classification;
DROP INDEX IF EXISTS idx_vb_assets_classification;

ALTER TABLE ck_policies DROP COLUMN IF EXISTS classification;
ALTER TABLE vb_assets   DROP COLUMN IF EXISTS classification;
