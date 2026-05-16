CREATE TABLE IF NOT EXISTS notification_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    notification_type TEXT NOT NULL,   -- 'breach_72h_warning' | 'dsr_overdue' | 'avv_expiring' | 'ccm_check_failed'
    resource_id     TEXT NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    recipient_email TEXT NOT NULL,
    UNIQUE (org_id, notification_type, resource_id, recipient_email)
);
CREATE INDEX IF NOT EXISTS notification_log_org_idx ON notification_log(org_id, sent_at DESC);
