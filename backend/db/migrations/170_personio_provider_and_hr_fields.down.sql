-- Migration 170 rollback

DROP INDEX IF EXISTS idx_hr_employees_personio_id;

ALTER TABLE hr_employees
  DROP COLUMN IF EXISTS personio_employee_id,
  DROP COLUMN IF EXISTS departure_date;

ALTER TABLE cloud_integrations
  DROP CONSTRAINT IF EXISTS cloud_integrations_provider_check;

ALTER TABLE cloud_integrations
  ADD CONSTRAINT cloud_integrations_provider_check
  CHECK (provider IN (
    'aws', 'azure',
    'hetzner', 'ionos',
    'wazuh', 'prometheus',
    'entra_id', 'keycloak',
    'gitlab', 'sonarqube',
    'ldap'
  ));
