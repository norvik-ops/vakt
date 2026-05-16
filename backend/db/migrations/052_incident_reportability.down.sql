ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS reportability_answers,
    DROP COLUMN IF EXISTS gdpr_notification_required;
