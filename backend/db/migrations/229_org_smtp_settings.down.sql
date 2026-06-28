ALTER TABLE organizations
  DROP COLUMN IF EXISTS smtp_host,
  DROP COLUMN IF EXISTS smtp_port,
  DROP COLUMN IF EXISTS smtp_user,
  DROP COLUMN IF EXISTS smtp_pass_enc,
  DROP COLUMN IF EXISTS smtp_from,
  DROP COLUMN IF EXISTS smtp_tls;
