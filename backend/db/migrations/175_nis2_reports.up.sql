-- S67-1: NIS2 Art.23 Meldepflicht-Workflow
-- Extends ck_incidents with stage-based reporting fields.
-- Adds ck_nis2_reports for storing form content per stage.
-- Adds ck_authority_contacts for DACH authority directory.

ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS nis2_reportable          BOOLEAN,
    ADD COLUMN IF NOT EXISTS nis2_reporting_stage     TEXT
        CHECK (nis2_reporting_stage IN ('none', 'early_warning', 'full_report', 'final_report'))
        DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS nis2_detected_at         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_early_warning_due   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_full_report_due     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_final_report_due    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_early_warning_submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_full_report_submitted_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS nis2_final_report_submitted_at  TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS ck_nis2_reports (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                   UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id              UUID        NOT NULL REFERENCES ck_incidents(id) ON DELETE CASCADE,
    stage                    TEXT        NOT NULL CHECK (stage IN ('early_warning', 'full_report', 'final_report')),
    affected_services        TEXT,
    initial_assessment       TEXT,
    root_cause               TEXT,
    affected_users_estimate  INTEGER,
    measures_taken           TEXT,
    estimated_recovery       TIMESTAMPTZ,
    full_root_cause_analysis TEXT,
    permanent_measures       TEXT,
    effectiveness_evidence   TEXT,
    submitted_by             UUID REFERENCES users(id),
    submitted_at             TIMESTAMPTZ,
    pdf_path                 TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (incident_id, stage)
);

CREATE INDEX IF NOT EXISTS idx_ck_nis2_reports_incident ON ck_nis2_reports (incident_id);

CREATE TABLE IF NOT EXISTS ck_authority_contacts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID        REFERENCES organizations(id) ON DELETE CASCADE,
    country        TEXT        NOT NULL CHECK (country IN ('de', 'at', 'ch', 'eu')),
    sector         TEXT,
    authority_name TEXT        NOT NULL,
    report_url     TEXT,
    email          TEXT,
    phone          TEXT,
    notes          TEXT,
    is_primary     BOOLEAN     NOT NULL DEFAULT false,
    is_builtin     BOOLEAN     NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ck_authority_contacts_org ON ck_authority_contacts (org_id, country);

-- Built-in DACH authority defaults (org_id NULL = shared across all orgs)
INSERT INTO ck_authority_contacts (country, authority_name, report_url, email, phone, notes, is_builtin)
VALUES
    ('de', 'BSI (allgemein)', 'https://meldung.bsi.bund.de', 'nis2@bsi.bund.de', '+49 228 9582-0',
     'Bundesamt für Sicherheit in der Informationstechnik — zuständig für die meisten NIS2-pflichtigen Einrichtungen', true),
    ('de', 'BNetzA (Energie/Telekommunikation)', 'https://www.bundesnetzagentur.de/nis2', NULL, '+49 228 14-0',
     'Bundesnetzagentur — sektorzuständig für Energie und Telekommunikation', true),
    ('de', 'BaFin (Finanzsektor)', 'https://www.bafin.de/dora', NULL, '+49 228 4108-0',
     'Bundesanstalt für Finanzdienstleistungsaufsicht — sektorzuständig für Finanzwesen/DORA', true),
    ('at', 'CERT.at', 'https://www.cert.at/de/meldungen/', 'incidents@cert.at', '+43 1 5056416 78',
     'Nationales CERT Österreich', true),
    ('ch', 'BACS', 'https://www.ncsc.admin.ch/meldung', 'meldungen@bacs.admin.ch', '+41 58 465 53 54',
     'Bundesamt für Cybersicherheit — zuständige Behörde in der Schweiz', true)
ON CONFLICT DO NOTHING;
