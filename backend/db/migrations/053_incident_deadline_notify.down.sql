ALTER TABLE ck_incidents
    DROP COLUMN IF EXISTS notified_warn_24h,
    DROP COLUMN IF EXISTS notified_warn_72h,
    DROP COLUMN IF EXISTS notified_warn_30d;
