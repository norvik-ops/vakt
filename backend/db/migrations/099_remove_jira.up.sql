-- Migration 099: Remove Jira integration
-- The Jira integration is being removed entirely. This migration drops the
-- jira_issues table and removes all Jira-specific columns from organisations.
-- This migration is intentionally irreversible (no down migration).

DROP TABLE IF EXISTS jira_issues;

ALTER TABLE organisations
  DROP COLUMN IF EXISTS jira_url,
  DROP COLUMN IF EXISTS jira_project_key,
  DROP COLUMN IF EXISTS jira_user_email,
  DROP COLUMN IF EXISTS jira_api_token;
