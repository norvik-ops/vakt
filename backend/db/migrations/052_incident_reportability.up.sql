-- 052: NIS2 reportability assessment fields for incidents (Story 31.1)
ALTER TABLE ck_incidents
    ADD COLUMN IF NOT EXISTS reportability_answers      JSONB,
    ADD COLUMN IF NOT EXISTS gdpr_notification_required BOOLEAN NOT NULL DEFAULT false;
