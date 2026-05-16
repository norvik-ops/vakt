-- Rotation policies
CREATE TABLE so_rotation_policies (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id            UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    secret_id         UUID NOT NULL REFERENCES so_secrets(id) ON DELETE CASCADE UNIQUE,
    interval_days     INT NOT NULL DEFAULT 90,
    last_rotated_at   TIMESTAMPTZ,
    next_rotation_at  TIMESTAMPTZ,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Git scan runs
CREATE TABLE so_git_scans (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    repo_url        TEXT NOT NULL,
    branch          TEXT NOT NULL DEFAULT 'main',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','completed','failed')),
    finding_count   INT NOT NULL DEFAULT 0,
    open_count      INT NOT NULL DEFAULT 0,
    dismissed_count INT NOT NULL DEFAULT 0,
    error_message   TEXT,
    scanned_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Git scan results (findings)
CREATE TABLE so_scan_results (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scan_id        UUID NOT NULL REFERENCES so_git_scans(id) ON DELETE CASCADE,
    repo_url       TEXT NOT NULL,
    commit_hash    TEXT,
    file_path      TEXT NOT NULL,
    line_number    INT,
    pattern_name   TEXT NOT NULL,
    match_preview  TEXT NOT NULL,
    severity       TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low')),
    status         TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','dismissed')),
    dismiss_reason TEXT,
    dismiss_count  INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_so_git_scans_org_id     ON so_git_scans(org_id);
CREATE INDEX idx_so_scan_results_scan_id ON so_scan_results(scan_id);
CREATE INDEX idx_so_rotation_secret_id   ON so_rotation_policies(secret_id);
