-- Migration 168 down: Revert provider CHECK to aws + azure only.
-- Only safe if no rows with new provider values exist.

ALTER TABLE cloud_integrations
  DROP CONSTRAINT IF EXISTS cloud_integrations_provider_check;

ALTER TABLE cloud_integrations
  ADD CONSTRAINT cloud_integrations_provider_check
  CHECK (provider IN ('aws', 'azure'));
