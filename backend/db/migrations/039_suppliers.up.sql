-- Lieferanten-Register: supplier directory for NIS2 Art. 21 / DORA Art. 28 supply chain compliance.
CREATE TABLE ck_suppliers (
  id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            TEXT        NOT NULL,
  contact_name    TEXT,
  contact_email   TEXT,
  service_type    TEXT,
  criticality     TEXT        NOT NULL DEFAULT 'standard',
  nis2_relevant   BOOLEAN     NOT NULL DEFAULT false,
  dora_relevant   BOOLEAN     NOT NULL DEFAULT false,
  contract_end    DATE,
  notes           TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ck_suppliers_org_id ON ck_suppliers (org_id);
