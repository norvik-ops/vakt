DROP TABLE IF EXISTS jira_issues;

ALTER TABLE organisations
  DROP COLUMN IF EXISTS jira_url,
  DROP COLUMN IF EXISTS jira_project_key,
  DROP COLUMN IF EXISTS jira_user_email,
  DROP COLUMN IF EXISTS jira_api_token;
