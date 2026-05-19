ALTER TABLE users
  ADD COLUMN IF NOT EXISTS preferred_language TEXT NOT NULL DEFAULT 'de'
    CHECK (preferred_language IN ('de', 'en', 'fr', 'nl'));
