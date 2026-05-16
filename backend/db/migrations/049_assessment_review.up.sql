ALTER TABLE ck_supplier_answers
  ADD COLUMN review_status TEXT CHECK (review_status IN ('accepted','needs_rework')),
  ADD COLUMN rework_note   TEXT,
  ADD COLUMN cert_expiry_date DATE;
