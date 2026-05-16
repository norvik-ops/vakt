-- 048_supplier_assessments.down.sql
DROP INDEX IF EXISTS idx_ck_supplier_answers_assessment;
DROP INDEX IF EXISTS idx_ck_supplier_assessments_org;
DROP INDEX IF EXISTS idx_ck_supplier_assessments_token;
DROP TABLE IF EXISTS ck_supplier_answers;
DROP TABLE IF EXISTS ck_supplier_assessments;
