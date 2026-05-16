-- Rollback migration 068: Policy Acceptance Campaigns

DROP INDEX IF EXISTS ck_policy_acceptance_token_idx;
DROP INDEX IF EXISTS ck_policy_acceptance_campaign_idx;
DROP TABLE IF EXISTS ck_policy_acceptance_requests;
DROP TABLE IF EXISTS ck_policy_acceptance_campaigns;
