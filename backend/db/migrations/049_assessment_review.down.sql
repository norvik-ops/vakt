ALTER TABLE ck_supplier_answers
  DROP COLUMN IF EXISTS cert_expiry_date,
  DROP COLUMN IF EXISTS rework_note,
  DROP COLUMN IF EXISTS review_status;
