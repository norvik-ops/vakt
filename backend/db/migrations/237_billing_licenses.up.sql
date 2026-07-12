-- Seat ledger: which licence keys were issued against which subscription.
--
-- An MSP buys 10 seats. A licence key carries the END CUSTOMER's organisation
-- name inside the signed payload, so 10 seats are 10 DIFFERENT keys — and at
-- purchase time the MSP does not yet know what its next 10 clients are called.
--
-- Nothing in a self-hosted instance can count activations: that would need
-- phone-home, which this product does not have and will not get. So a seat count
-- is an ENTITLEMENT, tracked here, not an enforcement. This table is the honest
-- version of that: it records what was actually handed out, so "8 of 10 used" is
-- a fact rather than a guess.
--
-- It doubles as the answer to "which key does customer X have?" — until now that
-- lived in exactly one place: the mail we sent them. If they lost it, so had we.

CREATE TABLE IF NOT EXISTS billing_licenses (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID        NOT NULL REFERENCES billing_quote_requests (id) ON DELETE CASCADE,

    -- The organisation the key is bound to. For a direct customer this is their
    -- own company; for an MSP it is one of their clients.
    org_name        TEXT        NOT NULL,

    license_key     TEXT        NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,

    -- 'trial'  — the 45-day key issued when the invoice went out
    -- 'full'   — issued once the money landed
    -- 'seat'   — an MSP seat, issued on request as they onboard a client
    kind            TEXT        NOT NULL DEFAULT 'full',

    note            TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_licenses_subscription
    ON billing_licenses (subscription_id);

-- Backfill: the key currently on the subscription row is the one that was mailed.
INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind, note)
SELECT id,
       company_name,
       license_key,
       -- The exact expiry is inside the signed key and not worth parsing here.
       -- Approximate from the interval so the ledger is not empty; the key itself
       -- remains the source of truth for when it actually runs out.
       COALESCE(paid_at, approved_at, created_at)
           + (CASE WHEN interval = 'year' THEN INTERVAL '395 days' ELSE INTERVAL '35 days' END),
       CASE WHEN status = 'paid' THEN 'full' ELSE 'trial' END,
       'backfilled from the subscription row (migration 237)'
  FROM billing_quote_requests
 WHERE license_key IS NOT NULL AND license_key <> '';
