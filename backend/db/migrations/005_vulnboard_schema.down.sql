-- Reverse VulnBoard schema migration
DROP INDEX IF EXISTS idx_vb_assets_tags;
DROP INDEX IF EXISTS idx_vb_assets_org_id;

DROP TABLE IF EXISTS vb_sla_config;
DROP TABLE IF EXISTS vb_assets;
