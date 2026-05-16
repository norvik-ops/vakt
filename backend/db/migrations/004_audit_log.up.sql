-- Audit log: immutable record of all mutating actions
-- NOTE: no UPDATE or DELETE queries must ever be issued against this table
CREATE TABLE audit_logs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    action         TEXT NOT NULL,
    resource_type  TEXT NOT NULL,
    resource_id    TEXT,
    ip_address     TEXT,
    user_agent     TEXT,
    status_code    INT,
    request_body   JSONB,
    timestamp      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_org_id    ON audit_logs(org_id);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp);
CREATE INDEX idx_audit_logs_user_id   ON audit_logs(user_id);
