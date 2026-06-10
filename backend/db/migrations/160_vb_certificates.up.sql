CREATE TABLE IF NOT EXISTS vb_certificates (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    domain          TEXT        NOT NULL,
    issuer          TEXT        NOT NULL DEFAULT '',
    subject         TEXT        NOT NULL DEFAULT '',
    sans            TEXT[]      NOT NULL DEFAULT '{}',
    not_before      TIMESTAMPTZ,
    not_after       TIMESTAMPTZ,
    asset_id        UUID        REFERENCES vb_assets(id) ON DELETE SET NULL,
    source          TEXT        NOT NULL DEFAULT 'manual'
                                CHECK (source IN ('manual', 'scan')),
    status          TEXT        NOT NULL DEFAULT 'unknown'
                                CHECK (status IN ('valid', 'expiring', 'expired', 'error', 'unknown')),
    last_checked_at TIMESTAMPTZ,
    error_msg       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vb_certs_org ON vb_certificates(org_id, not_after);
CREATE UNIQUE INDEX IF NOT EXISTS idx_vb_certs_org_domain ON vb_certificates(org_id, domain);
