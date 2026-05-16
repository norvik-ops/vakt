-- 053: Dedup guards for 12h-before-deadline warning notifications (Story 31.2)
ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS notified_warn_24h BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS notified_warn_72h BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS notified_warn_30d BOOLEAN NOT NULL DEFAULT false;
