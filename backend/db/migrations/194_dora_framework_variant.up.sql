ALTER TABLE ck_frameworks
  ADD COLUMN framework_variant TEXT NOT NULL DEFAULT 'full'
    CHECK (framework_variant IN ('full', 'simplified'));
