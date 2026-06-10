-- S69-1: Cross-Framework Prerequisite Chains + mapping_type extension
-- New table for intra- and cross-framework prerequisite relationships.

-- 1. Extend mapping_type constraint to include 'prerequisite'
ALTER TABLE ck_framework_control_mappings
    DROP CONSTRAINT IF EXISTS ck_framework_control_mappings_mapping_type_check;

ALTER TABLE ck_framework_control_mappings
    ADD CONSTRAINT ck_framework_control_mappings_mapping_type_check
        CHECK (mapping_type IN ('equivalent', 'partial', 'informative', 'prerequisite'));

-- 2. New table for prerequisite chains
CREATE TABLE IF NOT EXISTS ck_control_prerequisites (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    control_framework       TEXT        NOT NULL,
    control_code            TEXT        NOT NULL,
    prerequisite_framework  TEXT        NOT NULL,
    prerequisite_code       TEXT        NOT NULL,
    dependency_type         TEXT        NOT NULL CHECK (dependency_type IN (
        'required',     -- Without prerequisite, implementation is invalid/meaningless
        'recommended',  -- Strongly recommended but technically possible without
        'informative'   -- Reference/advisory, no blocking character
    )),
    rationale               TEXT,
    source                  TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(control_framework, control_code, prerequisite_framework, prerequisite_code)
);

CREATE INDEX IF NOT EXISTS idx_ck_prerequisites_control
    ON ck_control_prerequisites (control_framework, control_code);

CREATE INDEX IF NOT EXISTS idx_ck_prerequisites_prereq
    ON ck_control_prerequisites (prerequisite_framework, prerequisite_code);
