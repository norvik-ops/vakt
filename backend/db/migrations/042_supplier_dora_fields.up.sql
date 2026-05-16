ALTER TABLE ck_suppliers
  ADD COLUMN IF NOT EXISTS sub_suppliers TEXT[] DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS data_location TEXT CHECK (data_location IN ('EU', 'NonEU', 'Hybrid')),
  ADD COLUMN IF NOT EXISTS exit_strategy_exists BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE ck_incidents
  ADD COLUMN IF NOT EXISTS supplier_id UUID REFERENCES ck_suppliers(id) ON DELETE SET NULL;
