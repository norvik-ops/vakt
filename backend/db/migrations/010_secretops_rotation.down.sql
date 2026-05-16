DROP INDEX IF EXISTS idx_so_rotation_secret_id;
DROP INDEX IF EXISTS idx_so_scan_results_scan_id;
DROP INDEX IF EXISTS idx_so_git_scans_org_id;

DROP TABLE IF EXISTS so_scan_results;
DROP TABLE IF EXISTS so_git_scans;
DROP TABLE IF EXISTS so_rotation_policies;
