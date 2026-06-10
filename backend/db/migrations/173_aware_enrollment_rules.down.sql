ALTER TABLE sr_campaigns DROP COLUMN IF EXISTS enrollment_source;

DROP INDEX IF EXISTS idx_sr_campaign_enrollments_campaign;
DROP TABLE IF EXISTS sr_campaign_enrollments;

DROP INDEX IF EXISTS idx_sr_enrollment_rules_org;
DROP TABLE IF EXISTS sr_enrollment_rules;
