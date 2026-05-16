ALTER TABLE ck_controls
  ADD COLUMN manual_status TEXT;
-- null = computed from evidence, 'in_progress' = in Bearbeitung, 'implemented' = Umgesetzt (Selbstzertifizierung)
