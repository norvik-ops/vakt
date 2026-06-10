ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS inherent_likelihood INTEGER CHECK (inherent_likelihood BETWEEN 1 AND 5);
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS inherent_impact INTEGER CHECK (inherent_impact BETWEEN 1 AND 5);
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS residual_likelihood INTEGER CHECK (residual_likelihood BETWEEN 1 AND 5);
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS residual_impact INTEGER CHECK (residual_impact BETWEEN 1 AND 5);
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS risk_accepted_by UUID REFERENCES users(id);
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS risk_accepted_at TIMESTAMPTZ;
ALTER TABLE ck_risks ADD COLUMN IF NOT EXISTS risk_acceptance_justification TEXT NOT NULL DEFAULT '';
