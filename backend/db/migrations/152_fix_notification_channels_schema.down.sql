ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS config JSONB NOT NULL DEFAULT '{}';
ALTER TABLE notification_channels DROP COLUMN IF EXISTS events;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS url_encrypted;
ALTER TABLE notification_channels RENAME COLUMN enabled TO is_active;
ALTER TABLE notification_channels RENAME COLUMN type TO channel;
