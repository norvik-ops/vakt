ALTER TABLE ck_incidents
  DROP COLUMN IF EXISTS incident_type,
  DROP COLUMN IF EXISTS reporting_obligation,
  DROP COLUMN IF EXISTS notification_authority,
  DROP COLUMN IF EXISTS deadline_4h,
  DROP COLUMN IF EXISTS deadline_24h,
  DROP COLUMN IF EXISTS deadline_72h,
  DROP COLUMN IF EXISTS deadline_30d,
  DROP COLUMN IF EXISTS reported_4h_at,
  DROP COLUMN IF EXISTS reported_24h_at,
  DROP COLUMN IF EXISTS reported_72h_at,
  DROP COLUMN IF EXISTS reported_30d_at;
