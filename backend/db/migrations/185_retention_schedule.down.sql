DROP TABLE IF EXISTS po_deletion_reminders;
DROP TABLE IF EXISTS po_retention_templates;

ALTER TABLE po_processing_activities
    DROP COLUMN IF EXISTS retention_period_months,
    DROP COLUMN IF EXISTS retention_type,
    DROP COLUMN IF EXISTS retention_event_description,
    DROP COLUMN IF EXISTS retention_max_period_months,
    DROP COLUMN IF EXISTS deletion_method,
    DROP COLUMN IF EXISTS retention_legal_basis;
