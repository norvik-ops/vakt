DROP TABLE IF EXISTS ck_control_prerequisites;

ALTER TABLE ck_framework_control_mappings
    DROP CONSTRAINT IF EXISTS ck_framework_control_mappings_mapping_type_check;

ALTER TABLE ck_framework_control_mappings
    ADD CONSTRAINT ck_framework_control_mappings_mapping_type_check
        CHECK (mapping_type IN ('equivalent', 'partial', 'informative'));
