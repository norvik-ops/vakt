CREATE TABLE sr_enrollment_rules (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  trigger_type     TEXT NOT NULL CHECK (trigger_type IN ('new_employee', 'phishing_click')),
  target_campaign_id UUID REFERENCES sr_campaigns(id) ON DELETE SET NULL,
  is_active        BOOLEAN NOT NULL DEFAULT true,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sr_enrollment_rules_org ON sr_enrollment_rules (org_id, is_active);

CREATE TABLE sr_campaign_enrollments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID NOT NULL,
  campaign_id UUID NOT NULL REFERENCES sr_campaigns(id) ON DELETE CASCADE,
  employee_id TEXT NOT NULL,
  source      TEXT NOT NULL DEFAULT 'manual'
              CHECK (source IN ('manual', 'auto_new_employee', 'auto_phishing_click')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (campaign_id, employee_id)
);

CREATE INDEX idx_sr_campaign_enrollments_campaign ON sr_campaign_enrollments (campaign_id);
