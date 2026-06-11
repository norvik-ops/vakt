-- S74-4: BSI Referenzberichte A1–A6 — Audit-Log für generierte PDFs
CREATE TABLE ck_bsi_report_exports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    report_type     TEXT NOT NULL CHECK (report_type IN ('A1','A2','A3','A4','A5','A6','full')),
    generated_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    generated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sha256          TEXT NOT NULL DEFAULT '',
    file_size_bytes INTEGER,
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_bsi_report_exports_org ON ck_bsi_report_exports(org_id, generated_at DESC);
