-- Drop ComplyKit tables in reverse dependency order
DROP INDEX IF EXISTS idx_ck_reviews_control;
DROP INDEX IF EXISTS idx_ck_evidence_org_id;
DROP INDEX IF EXISTS idx_ck_evidence_control;
DROP INDEX IF EXISTS idx_ck_controls_framework;

DROP TABLE IF EXISTS ck_auditor_links;
DROP TABLE IF EXISTS ck_reviews;
DROP TABLE IF EXISTS ck_evidence;
DROP TABLE IF EXISTS ck_controls;
DROP TABLE IF EXISTS ck_frameworks;
