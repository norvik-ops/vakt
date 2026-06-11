DROP INDEX IF EXISTS idx_bsi_modeling_target;
ALTER TABLE ck_bsi_modeling DROP COLUMN IF EXISTS target_object_id;
DROP INDEX IF EXISTS idx_bsi_check_results_baustein;
DROP INDEX IF EXISTS idx_bsi_check_results_target;
DROP INDEX IF EXISTS idx_bsi_check_results_org;
DROP TABLE IF EXISTS ck_bsi_check_results;
DROP INDEX IF EXISTS idx_bsi_target_objects_org;
DROP TABLE IF EXISTS ck_bsi_target_objects;
