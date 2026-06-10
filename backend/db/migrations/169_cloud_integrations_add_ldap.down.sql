-- Revert migration 169: remove ldap from provider check

ALTER TABLE cloud_integrations
  DROP CONSTRAINT IF EXISTS cloud_integrations_provider_check;

ALTER TABLE cloud_integrations
  ADD CONSTRAINT cloud_integrations_provider_check
  CHECK (provider IN (
    'aws', 'azure',
    'hetzner', 'ionos',
    'wazuh', 'prometheus',
    'entra_id', 'keycloak',
    'gitlab', 'sonarqube'
  ));
