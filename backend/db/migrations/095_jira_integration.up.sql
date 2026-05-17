-- Migration 095: Jira integration
-- Adds Jira config columns to organisations and a tracking table for created issues.

ALTER TABLE organisations
  ADD COLUMN IF NOT EXISTS jira_url          TEXT,
  ADD COLUMN IF NOT EXISTS jira_project_key  TEXT,
  ADD COLUMN IF NOT EXISTS jira_user_email   TEXT,
  ADD COLUMN IF NOT EXISTS jira_api_token    TEXT;  -- stored AES-256-GCM encrypted (hex)

CREATE TABLE IF NOT EXISTS jira_issues (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          UUID        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  finding_id      UUID        NOT NULL,
  jira_issue_key  TEXT        NOT NULL,
  jira_issue_url  TEXT        NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (org_id, finding_id)
);

CREATE INDEX IF NOT EXISTS jira_issues_org_id_idx ON jira_issues (org_id);
