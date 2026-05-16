-- ComplyKit schema (ck_ prefix)

-- Compliance frameworks (NIS2, ISO27001, BSI, custom)
CREATE TABLE ck_frameworks (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT '1.0',
    is_builtin  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, name)
);

-- Control library: individual controls per framework
CREATE TABLE ck_controls (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    framework_id    UUID NOT NULL REFERENCES ck_frameworks(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    control_id      TEXT NOT NULL,  -- e.g. "NIS2-5.1"
    title           TEXT NOT NULL,
    description     TEXT,
    domain          TEXT NOT NULL,  -- e.g. "Access Control", "Risk Management"
    evidence_type   TEXT NOT NULL DEFAULT 'manual',  -- 'manual', 'automated', 'third_party'
    weight          INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(framework_id, control_id)
);

-- Evidence items: files or automated collector results
CREATE TABLE ck_evidence (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    control_id      UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    source          TEXT NOT NULL DEFAULT 'manual',  -- 'manual', 'github', 'aws', 'azure', 'ad'
    file_path       TEXT,  -- stored file path (if manual upload)
    file_size       BIGINT,
    collector_data  JSONB,  -- raw data from automated collector
    status          TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'approved', 'rejected', 'expired'
    version         INT NOT NULL DEFAULT 1,
    expires_at      TIMESTAMPTZ,
    uploaded_by     UUID REFERENCES users(id),
    reviewed_by     UUID REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Control review assignments
CREATE TABLE ck_reviews (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    control_id      UUID NOT NULL REFERENCES ck_controls(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    assigned_to     UUID NOT NULL REFERENCES users(id),
    assigned_by     UUID NOT NULL REFERENCES users(id),
    due_date        TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'in_review', 'approved', 'rejected'
    notes           TEXT,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Auditor access links
CREATE TABLE ck_auditor_links (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    framework_id UUID NOT NULL REFERENCES ck_frameworks(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    created_by   UUID NOT NULL REFERENCES users(id),
    expires_at   TIMESTAMPTZ NOT NULL,
    used_count   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_controls_framework ON ck_controls(framework_id);
CREATE INDEX idx_ck_evidence_control   ON ck_evidence(control_id);
CREATE INDEX idx_ck_evidence_org_id    ON ck_evidence(org_id);
CREATE INDEX idx_ck_reviews_control    ON ck_reviews(control_id);
