-- Tracks when an alert was last fired per (org, event_type) to prevent
-- repeated firing on every cron run for persistent overdue states.
CREATE TABLE IF NOT EXISTS notification_alert_state (
    org_id      UUID        NOT NULL,
    event_type  TEXT        NOT NULL,
    last_fired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, event_type)
);
