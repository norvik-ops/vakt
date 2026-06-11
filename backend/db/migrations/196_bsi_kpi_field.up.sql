-- S74-2: BSI Grundschutz-Cockpit — KPI-Snapshot-Feld
ALTER TABLE ck_isms_kpi_snapshots
    ADD COLUMN IF NOT EXISTS bsi_check_pct NUMERIC(5,2);
