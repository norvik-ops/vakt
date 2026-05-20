-- 122 down: rückbenennen sr_* → pg_*.
ALTER INDEX IF EXISTS sr_phish_reports_org_idx RENAME TO pg_phish_reports_org_idx;
ALTER INDEX idx_sr_events_token          RENAME TO idx_pg_events_token;
ALTER INDEX idx_sr_campaigns_org_id      RENAME TO idx_pg_campaigns_org_id;
ALTER INDEX idx_sr_targets_group_id      RENAME TO idx_pg_targets_group_id;
ALTER INDEX idx_sr_events_target_id      RENAME TO idx_pg_events_target_id;
ALTER INDEX idx_sr_events_campaign_id    RENAME TO idx_pg_events_campaign_id;
ALTER INDEX idx_sr_assignments_module_id RENAME TO idx_pg_assignments_module_id;
ALTER INDEX idx_sr_assignments_target_id RENAME TO idx_pg_assignments_target_id;

ALTER TABLE sr_phish_reports    RENAME TO pg_phish_reports;
ALTER TABLE sr_completions      RENAME TO pg_completions;
ALTER TABLE sr_assignments      RENAME TO pg_assignments;
ALTER TABLE sr_training_modules RENAME TO pg_training_modules;
ALTER TABLE sr_events           RENAME TO pg_events;
ALTER TABLE sr_campaigns        RENAME TO pg_campaigns;
ALTER TABLE sr_landing_pages    RENAME TO pg_landing_pages;
ALTER TABLE sr_targets          RENAME TO pg_targets;
ALTER TABLE sr_target_groups    RENAME TO pg_target_groups;
ALTER TABLE sr_templates        RENAME TO pg_templates;
