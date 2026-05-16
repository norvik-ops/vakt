CREATE TABLE bsi_advisories (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bsi_id       TEXT NOT NULL UNIQUE,   -- z.B. "WID-SEC-2024-1234"
    title        TEXT NOT NULL,
    summary      TEXT,
    severity     TEXT NOT NULL DEFAULT 'medium', -- critical/high/medium/low
    published_at TIMESTAMPTZ NOT NULL,
    url          TEXT,
    cve_ids      TEXT[] NOT NULL DEFAULT '{}',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_bsi_advisories_published ON bsi_advisories(published_at DESC);
