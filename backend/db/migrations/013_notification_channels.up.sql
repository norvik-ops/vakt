-- E01c: Notification channel configuration
-- Stores admin-configured delivery channels for outbound notifications.

CREATE TABLE notification_channels (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    channel     TEXT NOT NULL CHECK (channel IN ('slack', 'teams', 'email', 'webhook')),
    config      JSONB NOT NULL DEFAULT '{}',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, name)
);

CREATE INDEX idx_notification_channels_org_id ON notification_channels(org_id);
