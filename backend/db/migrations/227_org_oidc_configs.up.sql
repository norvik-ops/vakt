-- S105-2: Per-org OIDC/Casdoor configuration stored in DB (replaces env-var-only approach).
-- client_secret is encrypted at the application layer with VAKT_SECRET_KEY (AES-256-GCM).
CREATE TABLE org_oidc_configs (
    org_id              UUID        PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    provider_url        TEXT        NOT NULL,
    client_id           TEXT        NOT NULL,
    client_secret_enc   BYTEA       NOT NULL,
    enabled             BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
