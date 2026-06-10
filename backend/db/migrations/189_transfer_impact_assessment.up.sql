-- S69-6: Transfer Impact Assessment / TIA (Schrems II, DSGVO Art. 46)
-- Tracks third-country data transfers and associated TIA documents.

-- Static adequacy decision table (system-global)
CREATE TABLE IF NOT EXISTS po_adequacy_decisions (
    country_code        TEXT        PRIMARY KEY,
    country_name        TEXT        NOT NULL,
    has_adequacy        BOOLEAN     NOT NULL,
    decision_date       DATE,
    decision_reference  TEXT,
    notes               TEXT,
    last_updated        DATE        NOT NULL DEFAULT CURRENT_DATE
);

INSERT INTO po_adequacy_decisions VALUES
    ('GB', 'Vereinigtes Königreich', true,  '2021-06-28', 'EU 2021/1772', NULL, CURRENT_DATE),
    ('US', 'USA', true,  '2023-07-10', 'EU 2023/1795', 'Data Privacy Framework — politisch unsicher', CURRENT_DATE),
    ('CH', 'Schweiz', true,  '2000-07-26', NULL, 'Aktualisierung erwartet', CURRENT_DATE),
    ('JP', 'Japan', true,  '2019-01-23', NULL, NULL, CURRENT_DATE),
    ('CA', 'Kanada', true,  '2001-12-20', NULL, 'Nur kommerzielle Organisationen', CURRENT_DATE),
    ('IL', 'Israel', true,  '2011-01-31', NULL, NULL, CURRENT_DATE),
    ('NZ', 'Neuseeland', true,  '2012-12-19', NULL, NULL, CURRENT_DATE),
    ('KR', 'Südkorea', true,  '2021-12-17', NULL, NULL, CURRENT_DATE),
    ('AR', 'Argentinien', true,  '2003-06-30', NULL, NULL, CURRENT_DATE),
    ('UY', 'Uruguay', true,  '2012-08-21', NULL, NULL, CURRENT_DATE),
    ('AD', 'Andorra', true,  '2010-10-19', NULL, NULL, CURRENT_DATE),
    ('FO', 'Färöer', true,  '2010-03-05', NULL, NULL, CURRENT_DATE),
    ('GG', 'Guernsey', true,  '2003-11-21', NULL, NULL, CURRENT_DATE),
    ('IM', 'Isle of Man', true,  '2004-04-28', NULL, NULL, CURRENT_DATE),
    ('JE', 'Jersey', true,  '2008-05-08', NULL, NULL, CURRENT_DATE),
    ('CN', 'China', false, NULL, NULL, 'Kein Angemessenheitsbeschluss; NSL + PIPL-Konflikt mit DSGVO', CURRENT_DATE),
    ('RU', 'Russland', false, NULL, NULL, NULL, CURRENT_DATE),
    ('IN', 'Indien', false, NULL, NULL, 'DPDPA 2023 noch nicht als äquivalent anerkannt', CURRENT_DATE),
    ('BR', 'Brasilien', false, NULL, NULL, 'LGPD läuft — Angemessenheitsbeschluss möglich ab 2025', CURRENT_DATE)
ON CONFLICT DO NOTHING;

-- Per-org transfer records
CREATE TABLE IF NOT EXISTS po_data_transfers (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    processing_activity_id      UUID        REFERENCES po_vvt_entries(id) ON DELETE SET NULL,
    recipient_name              TEXT        NOT NULL,
    recipient_country           TEXT        NOT NULL,
    recipient_country_name      TEXT        NOT NULL,
    data_categories             TEXT[]      NOT NULL DEFAULT '{}',
    transfer_mechanism          TEXT        NOT NULL CHECK (transfer_mechanism IN (
        'adequacy_decision',
        'scc',
        'bcr',
        'derogation',
        'other'
    )),
    scc_version                 TEXT,
    status                      TEXT        NOT NULL CHECK (status IN (
        'adequate',
        'requires_tia',
        'tia_adequate',
        'tia_adequate_measures',
        'tia_inadequate',
        'under_review'
    )) DEFAULT 'requires_tia',
    is_active                   BOOLEAN     NOT NULL DEFAULT true,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_po_transfers_org     ON po_data_transfers (org_id, status);
CREATE INDEX IF NOT EXISTS idx_po_transfers_country ON po_data_transfers (recipient_country);

-- TIA documents
CREATE TABLE IF NOT EXISTS po_transfer_impact_assessments (
    id                              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    transfer_id                     UUID        NOT NULL REFERENCES po_data_transfers(id) ON DELETE CASCADE,
    legal_system_notes              TEXT        NOT NULL,
    surveillance_risk               TEXT        NOT NULL CHECK (surveillance_risk IN ('low', 'medium', 'high')),
    data_subject_rights_available   BOOLEAN     NOT NULL,
    encryption_in_transit           BOOLEAN     NOT NULL DEFAULT false,
    encryption_at_rest              BOOLEAN     NOT NULL DEFAULT false,
    pseudonymization_applied        BOOLEAN     NOT NULL DEFAULT false,
    access_controls_documented      BOOLEAN     NOT NULL DEFAULT false,
    supplementary_measures          TEXT,
    outcome                         TEXT        NOT NULL CHECK (outcome IN (
        'adequate',
        'adequate_with_measures',
        'inadequate'
    )),
    reviewed_by                     UUID        REFERENCES users(id),
    reviewed_at                     TIMESTAMPTZ,
    valid_until                     DATE,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_po_tia_transfer ON po_transfer_impact_assessments (transfer_id);
CREATE INDEX IF NOT EXISTS idx_po_tia_org      ON po_transfer_impact_assessments (org_id);
