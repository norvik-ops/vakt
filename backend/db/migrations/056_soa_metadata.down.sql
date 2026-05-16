ALTER TABLE ck_controls
    DROP COLUMN IF EXISTS soa_justification,
    DROP COLUMN IF EXISTS soa_implementation,
    DROP COLUMN IF EXISTS soa_responsible;
