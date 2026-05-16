-- 054: Archive table for generated NIS2/DORA incident report forms (Story 31.3)
CREATE TABLE IF NOT EXISTS ck_incident_reports (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id       UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id  UUID        NOT NULL REFERENCES ck_incidents(id) ON DELETE CASCADE,
    report_type  TEXT        NOT NULL,                             -- 24h | 72h | 30d
    authority    TEXT        NOT NULL DEFAULT 'BSI',
    pdf_data     BYTEA,
    metadata     JSONB,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_incident_reports_incident ON ck_incident_reports(incident_id);
