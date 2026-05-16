-- Migration 061: Phish-Button add-in support
-- Report-Phish records per org
CREATE TABLE IF NOT EXISTS pg_phish_reports (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    campaign_id    UUID REFERENCES pg_campaigns(id) ON DELETE SET NULL,
    reporter_email TEXT NOT NULL,
    reported_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    subject        TEXT,
    sender         TEXT,
    is_simulation  BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pg_phish_reports_org_idx ON pg_phish_reports(org_id, reported_at DESC);

-- Add report token to orgs for webhook auth
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS phish_report_token TEXT;
