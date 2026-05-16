-- SecretOps schema (so_ prefix)

-- Secret projects: top-level grouping
CREATE TABLE so_projects (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    description TEXT,
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

-- Environments per project (dev/staging/prod)
CREATE TABLE so_environments (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES so_projects(id) ON DELETE CASCADE,
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

-- Secrets: encrypted key-value pairs
CREATE TABLE so_secrets (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    environment_id   UUID NOT NULL REFERENCES so_environments(id) ON DELETE CASCADE,
    org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key              TEXT NOT NULL,
    encrypted_value  BYTEA NOT NULL,
    version          INT NOT NULL DEFAULT 1,
    rotation_due_at  TIMESTAMPTZ,
    last_rotated_at  TIMESTAMPTZ,
    last_accessed_at TIMESTAMPTZ,
    access_count     BIGINT NOT NULL DEFAULT 0,
    created_by       UUID NOT NULL REFERENCES users(id),
    updated_by       UUID REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(environment_id, key)
);

-- Secret access log
CREATE TABLE so_access_log (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    secret_id   UUID NOT NULL REFERENCES so_secrets(id) ON DELETE CASCADE,
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    accessed_by UUID REFERENCES users(id),
    access_via  TEXT NOT NULL, -- 'api', 'cli', 'sdk', 'share_link'
    ip_address  TEXT,
    user_agent  TEXT,
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Secret sharing links
CREATE TABLE so_share_links (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    secret_id   UUID NOT NULL REFERENCES so_secrets(id) ON DELETE CASCADE,
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Master key fingerprint (secret zero guard)
CREATE TABLE so_master_key_fingerprint (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    fingerprint TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_so_secrets_env_id       ON so_secrets(environment_id);
CREATE INDEX idx_so_secrets_org_id       ON so_secrets(org_id);
CREATE INDEX idx_so_access_log_secret_id ON so_access_log(secret_id);
CREATE INDEX idx_so_share_links_token_hash ON so_share_links(token_hash);
