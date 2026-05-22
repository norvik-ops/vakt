-- Migration 136: SCIM Token Management + SCIM provisioning metadata
-- S21-3 (SCIM 2.0 User+Group Provisioning) + S21-4 (SCIM Token Management)

-- scim_tokens: per-org Bearer tokens for the SCIM 2.0 provisioning endpoint.
-- The raw token is returned only once at creation time; only the sha256 hex
-- digest is stored so a DB leak does not expose usable tokens.
CREATE TABLE scim_tokens (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,   -- sha256 hex of the raw Bearer value
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX idx_scim_tokens_org_id      ON scim_tokens(org_id);
CREATE INDEX idx_scim_tokens_token_hash  ON scim_tokens(token_hash) WHERE revoked_at IS NULL;

-- scim_provisioned_source tracks which users were created/managed by SCIM
-- so that soft-deletes stay SCIM-scoped and don't affect manually created users.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS scim_external_id TEXT,
    ADD COLUMN IF NOT EXISTS scim_provisioned  BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_scim_external_id ON users(scim_external_id)
    WHERE scim_external_id IS NOT NULL;

-- scim_groups: maps IdP groups to Vakt roles per org.
-- SCIM Groups are org-scoped; the display_name is the IdP group name.
CREATE TABLE scim_groups (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name TEXT        NOT NULL,
    external_id  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, display_name)
);

CREATE INDEX idx_scim_groups_org_id ON scim_groups(org_id);

-- scim_group_members: many-to-many between scim_groups and users (within an org).
CREATE TABLE scim_group_members (
    group_id UUID NOT NULL REFERENCES scim_groups(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
