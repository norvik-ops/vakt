-- Migration 171: Add renewal_token and license_key to polar_subscriptions.
-- renewal_token is a stable UUID sent to the customer so their self-hosted
-- instance can poll GET /billing/license/:token for auto-renewal.
-- license_key stores the last issued key so the endpoint can serve it.

ALTER TABLE polar_subscriptions
    ADD COLUMN IF NOT EXISTS renewal_token UUID        NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS license_key   TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_polar_subscriptions_renewal_token
    ON polar_subscriptions (renewal_token);
