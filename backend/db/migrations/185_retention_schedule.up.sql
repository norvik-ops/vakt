-- S68-5: Löschfristen-Management (DSGVO Art. 5(1)(e), Art. 30 Abs.1(f))
-- Retention period tracking for VVT entries and deletion reminders.

ALTER TABLE po_vvt_entries
    ADD COLUMN IF NOT EXISTS retention_period_months       INTEGER,
    ADD COLUMN IF NOT EXISTS retention_type                TEXT
        CHECK (retention_type IN ('fixed', 'event_based', 'until_objection', 'permanent'))
        DEFAULT 'fixed',
    ADD COLUMN IF NOT EXISTS retention_event_description   TEXT,
    ADD COLUMN IF NOT EXISTS retention_max_period_months   INTEGER,
    ADD COLUMN IF NOT EXISTS deletion_method               TEXT
        CHECK (deletion_method IN (
            'secure_deletion', 'anonymization', 'physical_destroy', 'archival', 'other'
        )),
    ADD COLUMN IF NOT EXISTS retention_legal_basis         TEXT;

CREATE TABLE IF NOT EXISTS po_deletion_reminders (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                  UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    processing_activity_id  UUID        REFERENCES po_vvt_entries(id) ON DELETE CASCADE,
    description             TEXT        NOT NULL,
    data_category           TEXT,
    deletion_due_date       DATE        NOT NULL,
    reminder_sent_at        TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    completed_by            UUID        REFERENCES users(id),
    completion_notes        TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS po_retention_templates (
    id                     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    data_category          TEXT        NOT NULL,
    retention_period_months INTEGER,
    retention_type         TEXT,
    legal_basis            TEXT,
    notes                  TEXT,
    is_system_template     BOOLEAN     NOT NULL DEFAULT true
);

-- Standard DACH retention templates
INSERT INTO po_retention_templates (id, data_category, retention_period_months, retention_type, legal_basis, notes, is_system_template) VALUES
    (gen_random_uuid(), 'Bewerberdaten (abgelehnte Bewerber)', 6, 'fixed', 'AGG §15, allgemeine Grundsätze', NULL, true),
    (gen_random_uuid(), 'Mitarbeiter-Stammdaten', 120, 'fixed', 'HGB §257, AO §147', '10 Jahre nach Ausscheiden', true),
    (gen_random_uuid(), 'Gehaltsunterlagen / Lohnbelege', 120, 'fixed', 'AO §147', NULL, true),
    (gen_random_uuid(), 'Buchungsbelege, Rechnungen', 120, 'fixed', 'HGB §257', NULL, true),
    (gen_random_uuid(), 'Geschäftskommunikation (E-Mail, Briefe)', 72, 'fixed', 'HGB §257', NULL, true),
    (gen_random_uuid(), 'Videoüberwachung', NULL, 'fixed', 'BDSG §4', '72h bis max. 14 Tage', true),
    (gen_random_uuid(), 'IT-Log-Dateien', 6, 'fixed', 'BSI-Empfehlung', NULL, true),
    (gen_random_uuid(), 'Marketingkontakte', NULL, 'until_objection', 'DSGVO Art. 21', NULL, true)
ON CONFLICT DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_po_deletion_reminders_org  ON po_deletion_reminders (org_id, deletion_due_date)
    WHERE completed_at IS NULL;
