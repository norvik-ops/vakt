DROP TABLE IF EXISTS ck_authority_contacts;
DROP TABLE IF EXISTS ck_nis2_reports;

ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS nis2_reportable,
    DROP COLUMN IF EXISTS nis2_reporting_stage,
    DROP COLUMN IF EXISTS nis2_detected_at,
    DROP COLUMN IF EXISTS nis2_early_warning_due,
    DROP COLUMN IF EXISTS nis2_full_report_due,
    DROP COLUMN IF EXISTS nis2_final_report_due,
    DROP COLUMN IF EXISTS nis2_early_warning_submitted_at,
    DROP COLUMN IF EXISTS nis2_full_report_submitted_at,
    DROP COLUMN IF EXISTS nis2_final_report_submitted_at;
