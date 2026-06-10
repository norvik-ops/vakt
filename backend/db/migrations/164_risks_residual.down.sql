ALTER TABLE ck_risks DROP COLUMN IF EXISTS inherent_likelihood;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS inherent_impact;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS residual_likelihood;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS residual_impact;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS risk_accepted_by;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS risk_accepted_at;
ALTER TABLE ck_risks DROP COLUMN IF EXISTS risk_acceptance_justification;
