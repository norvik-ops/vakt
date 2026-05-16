-- VulnBoard schema (vb_ prefix)

-- Asset inventory
CREATE TABLE vb_assets (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL CHECK (type IN ('server','container','webapp','repository')),
    criticality  TEXT NOT NULL DEFAULT 'medium' CHECK (criticality IN ('low','medium','high','critical')),
    tags         TEXT[] NOT NULL DEFAULT '{}',
    owner_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    external_url TEXT,
    is_deleted   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- SLA configuration per organisation
CREATE TABLE vb_sla_config (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE UNIQUE,
    critical_days INT NOT NULL DEFAULT 7,
    high_days     INT NOT NULL DEFAULT 30,
    medium_days   INT NOT NULL DEFAULT 90,
    low_days      INT NOT NULL DEFAULT 180,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vb_assets_org_id ON vb_assets(org_id);
CREATE INDEX idx_vb_assets_tags   ON vb_assets USING GIN (tags);
