-- 056: Statement of Applicability (SoA) metadata fields for ISO 27001 controls
ALTER TABLE ck_controls
    ADD COLUMN IF NOT EXISTS soa_justification  TEXT,
    ADD COLUMN IF NOT EXISTS soa_implementation TEXT,
    ADD COLUMN IF NOT EXISTS soa_responsible    TEXT;
