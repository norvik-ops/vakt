ALTER TABLE organizations
  DROP COLUMN IF EXISTS backup_schedule,
  DROP COLUMN IF EXISTS backup_retention_days,
  DROP COLUMN IF EXISTS backup_passphrase_enc,
  DROP COLUMN IF EXISTS backup_notify_webhook_enc,
  DROP COLUMN IF EXISTS backup_offsite_cmd,
  DROP COLUMN IF EXISTS backup_notify_cmd;
