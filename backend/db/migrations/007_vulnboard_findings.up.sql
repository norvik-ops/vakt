-- Scan jobs table
CREATE TABLE vb_scans (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id      UUID NOT NULL REFERENCES vb_assets(id) ON DELETE CASCADE,
    scanner       TEXT NOT NULL CHECK (scanner IN ('trivy','nuclei','openvas')),
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','completed','failed')),
    target_url    TEXT,
    target_ip     TEXT,
    error_message TEXT,
    finding_count INT NOT NULL DEFAULT 0,
    duration_ms   BIGINT,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Findings table (normalized across all scanners)
CREATE TABLE vb_findings (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id         UUID NOT NULL REFERENCES vb_assets(id) ON DELETE CASCADE,
    scan_id          UUID REFERENCES vb_scans(id) ON DELETE SET NULL,
    cve_id           TEXT,
    title            TEXT NOT NULL,
    description      TEXT,
    severity         TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low','info')),
    cvss_score       NUMERIC(4,2),
    epss_score       NUMERIC(6,5),
    epss_percentile  NUMERIC(6,5),
    risk_score       NUMERIC(10,4),
    status           TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','resolved','accepted_risk','false_positive')),
    scanner          TEXT NOT NULL,
    raw_id           TEXT,
    sources          TEXT[] NOT NULL DEFAULT '{}',
    template_id      TEXT,
    assigned_to      UUID REFERENCES users(id) ON DELETE SET NULL,
    justification    TEXT,
    reopen_count     INT NOT NULL DEFAULT 0,
    occurrence_count INT NOT NULL DEFAULT 1,
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sla_due_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Finding suppression rules
CREATE TABLE vb_finding_suppressions (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cve_id     TEXT,
    asset_tag  TEXT,
    reason     TEXT NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    match_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Scan schedules
CREATE TABLE vb_scan_schedules (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    asset_id   UUID NOT NULL REFERENCES vb_assets(id) ON DELETE CASCADE,
    scanner    TEXT NOT NULL CHECK (scanner IN ('trivy','nuclei','openvas')),
    cron_expr  TEXT NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    last_run   TIMESTAMPTZ,
    next_run   TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Reports (generated executive summaries)
CREATE TABLE vb_reports (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    generated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    scope        JSONB NOT NULL DEFAULT '{}',
    file_path    TEXT,
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed','failed')),
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vb_scans_asset_id      ON vb_scans(asset_id);
CREATE INDEX idx_vb_scans_org_status    ON vb_scans(org_id, status);
CREATE INDEX idx_vb_findings_asset_id   ON vb_findings(asset_id);
CREATE INDEX idx_vb_findings_org_status ON vb_findings(org_id, status);
CREATE INDEX idx_vb_findings_cve        ON vb_findings(cve_id) WHERE cve_id IS NOT NULL;
CREATE INDEX idx_vb_findings_risk       ON vb_findings(org_id, risk_score DESC);
CREATE INDEX idx_vb_reports_org_id      ON vb_reports(org_id);
