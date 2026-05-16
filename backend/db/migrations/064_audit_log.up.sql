CREATE TABLE IF NOT EXISTS audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    user_email    TEXT,
    action        TEXT NOT NULL,       -- 'create' | 'update' | 'delete' | 'approve' | 'export'
    resource_type TEXT NOT NULL,       -- 'vvt' | 'dpia' | 'avv' | 'breach' | 'dsr' | 'control' | 'policy' | ...
    resource_id   TEXT,
    resource_name TEXT,
    details       JSONB,               -- optional: changed fields, old/new values
    ip_address    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_log_org_idx      ON audit_log(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_user_idx     ON audit_log(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_resource_idx ON audit_log(resource_type, resource_id);
