-- Fix notification_channels schema mismatch.
-- Migration 013 created the table with the old schema (channel / config / is_active).
-- Migration 025 used CREATE TABLE IF NOT EXISTS and silently no-op'd, leaving the
-- old columns in place while the code already referenced the new ones (type / url_encrypted
-- / events / enabled). This migration aligns the existing table with what the code expects.

ALTER TABLE notification_channels RENAME COLUMN channel TO type;
ALTER TABLE notification_channels RENAME COLUMN is_active TO enabled;
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS url_encrypted BYTEA NOT NULL DEFAULT ''::bytea;
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS events TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE notification_channels DROP COLUMN IF EXISTS config;
ALTER TABLE notification_channels ALTER COLUMN url_encrypted DROP DEFAULT;
