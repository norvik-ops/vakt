ALTER TABLE ck_controls
  ADD COLUMN not_applicable        BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN not_applicable_reason TEXT;
