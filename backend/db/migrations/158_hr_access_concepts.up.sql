CREATE TABLE hr_access_concepts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    title           TEXT NOT NULL,
    scope           TEXT NOT NULL DEFAULT '',
    owner           TEXT NOT NULL DEFAULT '',
    current_version INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hr_access_concepts_org_id ON hr_access_concepts (org_id);

CREATE TABLE hr_access_roles (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    concept_id              UUID NOT NULL REFERENCES hr_access_concepts(id) ON DELETE CASCADE,
    org_id                  UUID NOT NULL,
    role_name               TEXT NOT NULL,
    system_name             TEXT NOT NULL,
    access_level            TEXT NOT NULL CHECK (access_level IN ('read','write','admin','no_access')),
    justification           TEXT NOT NULL DEFAULT '',
    review_interval_months  INT NOT NULL DEFAULT 12,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hr_access_roles_concept_id ON hr_access_roles (concept_id);

CREATE TABLE hr_access_concept_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    concept_id      UUID NOT NULL REFERENCES hr_access_concepts(id) ON DELETE CASCADE,
    org_id          UUID NOT NULL,
    version_number  INT NOT NULL,
    snapshot        JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (concept_id, version_number)
);

CREATE INDEX idx_hr_access_concept_versions_concept_id ON hr_access_concept_versions (concept_id);
