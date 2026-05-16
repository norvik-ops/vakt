DROP INDEX IF EXISTS idx_vb_reports_org_id;
DROP INDEX IF EXISTS idx_vb_findings_risk;
DROP INDEX IF EXISTS idx_vb_findings_cve;
DROP INDEX IF EXISTS idx_vb_findings_org_status;
DROP INDEX IF EXISTS idx_vb_findings_asset_id;
DROP INDEX IF EXISTS idx_vb_scans_org_status;
DROP INDEX IF EXISTS idx_vb_scans_asset_id;

DROP TABLE IF EXISTS vb_reports;
DROP TABLE IF EXISTS vb_scan_schedules;
DROP TABLE IF EXISTS vb_finding_suppressions;
DROP TABLE IF EXISTS vb_findings;
DROP TABLE IF EXISTS vb_scans;
