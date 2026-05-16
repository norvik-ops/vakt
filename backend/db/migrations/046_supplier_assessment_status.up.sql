ALTER TABLE ck_suppliers
  ADD COLUMN IF NOT EXISTS assessment_status TEXT NOT NULL DEFAULT 'none' CHECK (assessment_status IN ('none','pending','completed')),
  ADD COLUMN IF NOT EXISTS last_assessment_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS ck_supplier_risks (
  supplier_id UUID NOT NULL REFERENCES ck_suppliers(id) ON DELETE CASCADE,
  risk_id     UUID NOT NULL REFERENCES ck_risks(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (supplier_id, risk_id)
);
