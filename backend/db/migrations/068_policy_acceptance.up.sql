-- Migration 068: Policy Acceptance Campaigns (ISO 27001 A.5.1 Evidence)

CREATE TABLE IF NOT EXISTS ck_policy_acceptance_campaigns (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    policy_id   UUID NOT NULL REFERENCES ck_policies(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    message     TEXT,
    deadline    DATE,
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ck_policy_acceptance_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id     UUID NOT NULL REFERENCES ck_policy_acceptance_campaigns(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    recipient_email TEXT NOT NULL,
    recipient_name  TEXT,
    token_hash      TEXT NOT NULL UNIQUE,
    accepted_at     TIMESTAMPTZ,
    accepted_ip     TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ck_policy_acceptance_campaign_idx ON ck_policy_acceptance_requests(campaign_id);
CREATE INDEX IF NOT EXISTS ck_policy_acceptance_token_idx   ON ck_policy_acceptance_requests(token_hash);
