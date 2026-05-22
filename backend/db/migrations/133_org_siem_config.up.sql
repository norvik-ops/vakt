CREATE TABLE IF NOT EXISTS org_siem_config (
    org_id       UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    enabled      BOOLEAN NOT NULL DEFAULT false,
    adapter      TEXT NOT NULL DEFAULT 'webhook', -- splunk_hec | elastic | webhook
    endpoint     TEXT NOT NULL DEFAULT '',
    token        TEXT NOT NULL DEFAULT '',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
