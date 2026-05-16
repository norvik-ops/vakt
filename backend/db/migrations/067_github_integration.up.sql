-- Migration 067: GitHub Integration
-- Stores GitHub repository integrations and their compliance check results.

CREATE TABLE IF NOT EXISTS integrations_github (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    repo_owner      TEXT NOT NULL,
    repo_name       TEXT NOT NULL,
    access_token    TEXT NOT NULL,       -- encrypted with AES-256-GCM (Platform Key)
    last_synced_at  TIMESTAMPTZ,
    sync_status     TEXT DEFAULT 'pending', -- pending | ok | error
    sync_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, repo_owner, repo_name)
);
CREATE INDEX IF NOT EXISTS integrations_github_org_idx ON integrations_github(org_id);

CREATE TABLE IF NOT EXISTS integrations_github_checks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    integration_id  UUID NOT NULL REFERENCES integrations_github(id) ON DELETE CASCADE,
    check_type      TEXT NOT NULL, -- 'branch_protection' | 'pr_review_required' | 'dependency_alerts' | 'secret_scanning'
    status          TEXT NOT NULL, -- 'pass' | 'fail' | 'unknown'
    details         JSONB,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS integrations_github_checks_integration_idx ON integrations_github_checks(integration_id, checked_at DESC);
