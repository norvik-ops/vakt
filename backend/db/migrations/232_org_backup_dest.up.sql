ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS backup_dest_type        TEXT,
  ADD COLUMN IF NOT EXISTS backup_dest_config_enc  BYTEA;
