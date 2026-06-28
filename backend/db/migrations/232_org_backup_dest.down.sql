ALTER TABLE organizations
  DROP COLUMN IF EXISTS backup_dest_type,
  DROP COLUMN IF EXISTS backup_dest_config_enc;
