DROP TABLE IF EXISTS ck_supplier_risks;
ALTER TABLE ck_suppliers DROP COLUMN IF EXISTS last_assessment_at;
ALTER TABLE ck_suppliers DROP COLUMN IF EXISTS assessment_status;
