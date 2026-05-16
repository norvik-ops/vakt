ALTER TABLE ck_incidents DROP COLUMN IF EXISTS supplier_id;
ALTER TABLE ck_suppliers DROP COLUMN IF EXISTS exit_strategy_exists;
ALTER TABLE ck_suppliers DROP COLUMN IF EXISTS data_location;
ALTER TABLE ck_suppliers DROP COLUMN IF EXISTS sub_suppliers;
