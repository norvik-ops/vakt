DROP INDEX IF EXISTS idx_sr_templates_category;

ALTER TABLE sr_templates
  DROP COLUMN IF EXISTS placeholders,
  DROP COLUMN IF EXISTS language,
  DROP COLUMN IF EXISTS difficulty,
  DROP COLUMN IF EXISTS category;
