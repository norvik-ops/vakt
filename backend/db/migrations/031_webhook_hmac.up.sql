ALTER TABLE notification_channels
  ADD COLUMN IF NOT EXISTS hmac_secret_encrypted BYTEA;
