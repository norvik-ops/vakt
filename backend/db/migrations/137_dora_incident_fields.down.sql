ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS dora_deadline_status,
    DROP COLUMN IF EXISTS dora_classification,
    DROP COLUMN IF EXISTS severity_dora,
    DROP COLUMN IF EXISTS authority,
    DROP COLUMN IF EXISTS first_detected_at;

-- Restore old check constraint (general | nis2 | dora)
ALTER TABLE ck_incidents
    DROP CONSTRAINT IF EXISTS ck_incidents_incident_type_check;

ALTER TABLE ck_incidents
    ADD CONSTRAINT ck_incidents_incident_type_check
    CHECK (incident_type IN ('general', 'nis2', 'dora'));
