ALTER TABLE sr_templates
  ADD COLUMN IF NOT EXISTS category      TEXT,
  ADD COLUMN IF NOT EXISTS difficulty    TEXT CHECK (difficulty IN ('easy', 'medium', 'hard')),
  ADD COLUMN IF NOT EXISTS language      TEXT NOT NULL DEFAULT 'de',
  ADD COLUMN IF NOT EXISTS placeholders  TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_sr_templates_category ON sr_templates (category) WHERE category IS NOT NULL;
