CREATE TABLE IF NOT EXISTS ck_capas (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL CHECK (source_type IN ('audit','incident','risk','manual')),
  source_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  root_cause TEXT NOT NULL DEFAULT '',
  action_plan TEXT NOT NULL DEFAULT '',
  assignee_email TEXT NOT NULL DEFAULT '',
  due_date DATE,
  priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low','medium','high','critical')),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','implemented','verified','closed')),
  verification_note TEXT NOT NULL DEFAULT '',
  closed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ck_capas_org ON ck_capas(org_id, status);
CREATE INDEX IF NOT EXISTS idx_ck_capas_source ON ck_capas(org_id, source_type, source_id);
