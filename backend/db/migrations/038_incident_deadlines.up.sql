-- NIS2 / DORA deadline tracking fields for incidents.
-- Deadlines are computed at creation time from discovered_at.
ALTER TABLE ck_incidents
  ADD COLUMN IF NOT EXISTS incident_type          TEXT        NOT NULL DEFAULT 'general',
  ADD COLUMN IF NOT EXISTS reporting_obligation   TEXT        NOT NULL DEFAULT 'unknown',
  ADD COLUMN IF NOT EXISTS notification_authority TEXT,
  ADD COLUMN IF NOT EXISTS deadline_4h            TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deadline_24h           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deadline_72h           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deadline_30d           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reported_4h_at         TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reported_24h_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reported_72h_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reported_30d_at        TIMESTAMPTZ;
