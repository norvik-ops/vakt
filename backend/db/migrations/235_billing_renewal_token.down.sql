-- Restores the structure of the two dead payment processors' tables (they held no
-- rows — verified in production before the drop) and removes the renewal token.

DROP INDEX IF EXISTS idx_billing_quote_requests_renewal_token;
ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS renewal_token;

CREATE TABLE IF NOT EXISTS ls_subscriptions (
    id                 BIGSERIAL   PRIMARY KEY,
    ls_subscription_id TEXT        NOT NULL UNIQUE,
    tier               TEXT        NOT NULL DEFAULT 'pro',
    status             TEXT        NOT NULL DEFAULT 'active',
    customer_email     TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ls_subscriptions_email ON ls_subscriptions (customer_email);

CREATE TABLE IF NOT EXISTS ls_revoked_subscriptions (
    org_id     UUID        PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    reason     TEXT        NOT NULL DEFAULT 'cancelled',
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lemonsqueezy_webhook_events (
    event_hash  TEXT        PRIMARY KEY,
    event_name  TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ls_webhook_events_received_at
    ON lemonsqueezy_webhook_events (received_at);

CREATE TABLE IF NOT EXISTS polar_subscriptions (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    polar_subscription_id TEXT        NOT NULL UNIQUE,
    customer_email        TEXT        NOT NULL,
    tier                  TEXT        NOT NULL DEFAULT 'pro',
    status                TEXT        NOT NULL DEFAULT 'active',
    renewal_token         UUID        NOT NULL DEFAULT gen_random_uuid(),
    license_key           TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_polar_subscriptions_email
    ON polar_subscriptions (customer_email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_polar_subscriptions_renewal_token
    ON polar_subscriptions (renewal_token);

CREATE TABLE IF NOT EXISTS polar_webhook_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_hash  TEXT        NOT NULL UNIQUE,
    event_type  TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
