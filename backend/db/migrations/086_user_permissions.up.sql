-- Migration 086: per-user module permission table
-- Enables granular read/write access control per module, per user, per org.

CREATE TABLE user_module_permissions (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  module      TEXT        NOT NULL CHECK (module IN ('vaktscan','vaktcomply','vaktvault','vaktaware','vaktprivacy')),
  can_read    BOOLEAN     NOT NULL DEFAULT true,
  can_write   BOOLEAN     NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (org_id, user_id, module)
);

CREATE INDEX idx_user_module_perms_user ON user_module_permissions(org_id, user_id);
