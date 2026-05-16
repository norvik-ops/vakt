-- 057: Trust Center — certificates, public policies and visibility toggles
ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS trust_center_logo_url        TEXT,
    ADD COLUMN IF NOT EXISTS trust_center_show_frameworks BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS trust_center_show_policies   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS trust_center_show_certs      BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS trust_center_subprocessors_md TEXT;

CREATE TABLE IF NOT EXISTS tc_certificates (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    issuer       TEXT,
    issued_at    DATE,
    expires_at   DATE,
    is_public    BOOLEAN NOT NULL DEFAULT TRUE,
    display_order INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tc_certs_org ON tc_certificates(org_id, display_order);

CREATE TABLE IF NOT EXISTS tc_public_policies (
    org_id    UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES ck_policies(id) ON DELETE CASCADE,
    PRIMARY KEY (org_id, policy_id)
);
