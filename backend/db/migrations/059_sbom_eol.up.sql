-- 059: SBOM generation and EOL component tracking (CRA readiness)
CREATE TABLE IF NOT EXISTS vb_sboms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id        UUID NOT NULL REFERENCES vb_assets(id) ON DELETE CASCADE,
    format          TEXT NOT NULL DEFAULT 'cyclonedx',
    spec_version    TEXT NOT NULL DEFAULT '1.4',
    document        JSONB NOT NULL DEFAULT '{}'::JSONB,
    component_count INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vb_sboms_asset ON vb_sboms(asset_id, created_at DESC);

CREATE TABLE IF NOT EXISTS vb_components (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    sbom_id      UUID NOT NULL REFERENCES vb_sboms(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    version      TEXT NOT NULL,
    purl         TEXT,
    eol_status   TEXT NOT NULL DEFAULT 'unknown' CHECK (eol_status IN ('supported','eol','unknown')),
    eol_date     DATE,
    eol_checked_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sbom_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_vb_components_eol ON vb_components(org_id, eol_status);

CREATE TABLE IF NOT EXISTS vb_eol_cache (
    product     TEXT NOT NULL,
    cycle       TEXT NOT NULL,
    eol_date    DATE,
    payload     JSONB,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product, cycle)
);
