-- S74-1: IT-Grundschutz-Check-Workflow
-- Zielobjekt-Inventar (Strukturanalyse) + Check-Ergebnistabelle

CREATE TABLE ck_bsi_target_objects (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    type                TEXT NOT NULL CHECK (type IN (
                            'it_system',
                            'application',
                            'network',
                            'room',
                            'process'
                        )),
    description         TEXT NOT NULL DEFAULT '',
    protection_c        TEXT CHECK (protection_c IN ('normal', 'hoch', 'sehr_hoch')),
    protection_i        TEXT CHECK (protection_i IN ('normal', 'hoch', 'sehr_hoch')),
    protection_a        TEXT CHECK (protection_a IN ('normal', 'hoch', 'sehr_hoch')),
    absicherungsniveau  TEXT NOT NULL DEFAULT 'standard'
                            CHECK (absicherungsniveau IN ('basis', 'standard', 'kern')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bsi_target_objects_org ON ck_bsi_target_objects(org_id);

CREATE TABLE ck_bsi_check_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    target_object_id    UUID NOT NULL REFERENCES ck_bsi_target_objects(id) ON DELETE CASCADE,
    baustein_id         TEXT NOT NULL,
    anforderung_id      TEXT NOT NULL,
    umsetzungsstatus    TEXT NOT NULL DEFAULT 'nein'
                            CHECK (umsetzungsstatus IN ('entbehrlich', 'ja', 'teilweise', 'nein')),
    begruendung         TEXT NOT NULL DEFAULT '',
    verantwortlicher    TEXT NOT NULL DEFAULT '',
    umsetzungsdatum     DATE,
    notiz               TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, target_object_id, anforderung_id)
);

CREATE INDEX idx_bsi_check_results_org      ON ck_bsi_check_results(org_id);
CREATE INDEX idx_bsi_check_results_target   ON ck_bsi_check_results(target_object_id);
CREATE INDEX idx_bsi_check_results_baustein ON ck_bsi_check_results(org_id, baustein_id);

ALTER TABLE ck_bsi_modeling
    ADD COLUMN target_object_id UUID REFERENCES ck_bsi_target_objects(id) ON DELETE SET NULL;

CREATE INDEX idx_bsi_modeling_target ON ck_bsi_modeling(target_object_id);
