ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS backup_schedule           TEXT,
  ADD COLUMN IF NOT EXISTS backup_retention_days     INT,
  ADD COLUMN IF NOT EXISTS backup_passphrase_enc     BYTEA,
  ADD COLUMN IF NOT EXISTS backup_notify_webhook_enc BYTEA,
  ADD COLUMN IF NOT EXISTS backup_offsite_cmd        TEXT,
  ADD COLUMN IF NOT EXISTS backup_notify_cmd         TEXT;
