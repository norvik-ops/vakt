-- Down migration for 071_risk_treatment
DROP TABLE IF EXISTS ck_risk_control_links;

ALTER TABLE ck_risks
  DROP COLUMN IF EXISTS treatment_option,
  DROP COLUMN IF EXISTS treatment_plan,
  DROP COLUMN IF EXISTS treatment_owner,
  DROP COLUMN IF EXISTS treatment_due_date,
  DROP COLUMN IF EXISTS treatment_status,
  DROP COLUMN IF EXISTS residual_likelihood,
  DROP COLUMN IF EXISTS residual_impact;
