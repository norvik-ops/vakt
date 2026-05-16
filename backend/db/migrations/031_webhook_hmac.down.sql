ALTER TABLE notification_channels
  DROP COLUMN IF EXISTS hmac_secret_encrypted;
