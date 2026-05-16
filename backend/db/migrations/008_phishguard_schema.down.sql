DROP INDEX IF EXISTS idx_pg_events_token;
DROP INDEX IF EXISTS idx_pg_campaigns_org_id;
DROP INDEX IF EXISTS idx_pg_targets_group_id;
DROP INDEX IF EXISTS idx_pg_events_target_id;
DROP INDEX IF EXISTS idx_pg_events_campaign_id;

DROP TABLE IF EXISTS pg_events;
DROP TABLE IF EXISTS pg_campaigns;
DROP TABLE IF EXISTS pg_landing_pages;
DROP TABLE IF EXISTS pg_targets;
DROP TABLE IF EXISTS pg_target_groups;
DROP TABLE IF EXISTS pg_templates;
