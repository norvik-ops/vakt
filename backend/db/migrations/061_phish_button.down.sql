-- Rollback migration 061
DROP TABLE IF EXISTS pg_phish_reports;
ALTER TABLE organizations DROP COLUMN IF EXISTS phish_report_token;
