-- Migration 148: Polar.sh subscription tracking + webhook deduplication.
-- Replaces LemonSqueezy as the payment processor for Vakt Pro license issuance.
-- ls_revoked_subscriptions is reused for revocation (processor-agnostic table).
CREATE TABLE IF NOT EXISTS polar_subscriptions (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    polar_subscription_id TEXT        NOT NULL UNIQUE,
    customer_email        TEXT        NOT NULL,
    tier                  TEXT        NOT NULL DEFAULT 'pro',
    status                TEXT        NOT NULL DEFAULT 'active',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_polar_subscriptions_email
    ON polar_subscriptions (customer_email);

CREATE TABLE IF NOT EXISTS polar_webhook_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_hash  TEXT        NOT NULL UNIQUE,
    event_type  TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
