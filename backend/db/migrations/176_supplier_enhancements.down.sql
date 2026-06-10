DROP INDEX IF EXISTS idx_ck_suppliers_assessment_due;

ALTER TABLE ck_suppliers
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS data_access,
    DROP COLUMN IF EXISTS avv_document_id,
    DROP COLUMN IF EXISTS last_assessment_score,
    DROP COLUMN IF EXISTS next_assessment_due,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS contract_start,
    DROP COLUMN IF EXISTS data_protection_score,
    DROP COLUMN IF EXISTS availability_score,
    DROP COLUMN IF EXISTS security_certifications,
    DROP COLUMN IF EXISTS audit_rights,
    DROP COLUMN IF EXISTS sub_processors_known,
    DROP COLUMN IF EXISTS incident_notification;
