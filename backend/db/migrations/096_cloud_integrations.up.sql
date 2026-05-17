-- Migration 096: Cloud integrations (AWS + Azure automated evidence collection)

CREATE TABLE IF NOT EXISTS cloud_integrations (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id           UUID        NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
  provider         TEXT        NOT NULL CHECK (provider IN ('aws', 'azure')),
  config           JSONB       NOT NULL DEFAULT '{}',  -- encrypted fields stored inside
  enabled          BOOLEAN     NOT NULL DEFAULT true,
  last_sync_at     TIMESTAMPTZ,
  last_sync_status TEXT,
  last_sync_error  TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (org_id, provider)
);

CREATE INDEX IF NOT EXISTS cloud_integrations_org_id_idx ON cloud_integrations (org_id);
CREATE INDEX IF NOT EXISTS cloud_integrations_enabled_idx ON cloud_integrations (enabled) WHERE enabled = true;
