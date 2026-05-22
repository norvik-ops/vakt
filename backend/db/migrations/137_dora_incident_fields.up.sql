-- S37-3: DORA IKT-Incident extended fields on ck_incidents.
-- Migration 038 already added: incident_type, deadline_*, reported_*
-- This migration adds DORA-specific severity, classification, and computed deadline status.

-- Extend the incident_type allowed values to include 'ikt_dora'.
-- Drop old constraint (if any) then add updated one.
ALTER TABLE ck_incidents
    DROP CONSTRAINT IF EXISTS ck_incidents_incident_type_check;

ALTER TABLE ck_incidents
    ADD CONSTRAINT ck_incidents_incident_type_check
    CHECK (incident_type IN ('general', 'nis2', 'dora', 'ikt_dora', 'dsgvo_breach'));

-- New DORA-specific fields not present in earlier migrations.
ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS first_detected_at    TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS authority            TEXT NULL,
    ADD COLUMN IF NOT EXISTS severity_dora        TEXT NULL
        CHECK (severity_dora IN ('schwerwiegend', 'erheblich', 'gering', NULL)),
    ADD COLUMN IF NOT EXISTS dora_classification  JSONB NULL,
    ADD COLUMN IF NOT EXISTS dora_deadline_status JSONB NULL;

CREATE INDEX IF NOT EXISTS idx_ck_incidents_type ON ck_incidents (incident_type);
CREATE INDEX IF NOT EXISTS idx_ck_incidents_dora_status ON ck_incidents
    USING GIN (dora_deadline_status) WHERE incident_type = 'ikt_dora';
