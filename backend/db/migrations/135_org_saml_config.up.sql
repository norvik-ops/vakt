-- S21-1: Per-org SAML 2.0 SP configuration for direct (non-Casdoor) IdP integration.
CREATE TABLE org_saml_configs (
    org_id          UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    entity_id       TEXT NOT NULL,          -- SP Entity ID (e.g. https://vakt.company.com/saml)
    acs_url         TEXT NOT NULL,          -- Assertion Consumer Service URL
    idp_metadata    TEXT NOT NULL,          -- IdP metadata XML (full blob stored here)
    cert_pem        TEXT NOT NULL,          -- SP signing/encryption cert (PEM)
    key_pem         TEXT NOT NULL,          -- SP private key (PEM, stored encrypted at app layer)
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
