-- Migration 170: Add personio provider + hr_employees offboarding fields for S64

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
    'ldap', 'personio'
  ));

ALTER TABLE hr_employees
  ADD COLUMN IF NOT EXISTS personio_employee_id INTEGER,
  ADD COLUMN IF NOT EXISTS departure_date DATE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_employees_personio_id
  ON hr_employees (org_id, personio_employee_id)
  WHERE personio_employee_id IS NOT NULL;
