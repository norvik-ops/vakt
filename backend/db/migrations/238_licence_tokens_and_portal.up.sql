-- The renewal token belongs to the LICENCE, not to the subscription.
--
-- Migration 235 hung it on billing_quote_requests, which is right for a direct
-- customer (one subscription, one key) and WRONG for an MSP. An MSP with ten seats
-- has ten DIFFERENT keys — each carries the end customer's organisation name in its
-- signed payload. With one token per subscription, all ten instances would have
-- polled GET /billing/license, matched the same row, and been handed the
-- subscription's single license_key: the MSP's own. Nine end customers would have
-- silently replaced their correct key with a stranger's.
--
-- Born broken. Never shipped — found before the first MSP existed.
--
-- Moving the token also buys the control that was missing, without breaking the
-- no-phone-home promise. That promise is about COMPLIANCE DATA, not about a licence
-- check: the instance already contacts api.norvikops.de daily (opt-in, token only).
-- With a token per licence, that call finally says something useful:
--
--   last_seen_at  which of the ten seats are actually running
--   revoked_at    stop renewing ONE seat without touching the other nine
--
-- Neither transmits a single byte of the customer's ISMS.

ALTER TABLE billing_licenses
    ADD COLUMN IF NOT EXISTS renewal_token UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS last_seen_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS revoked_at    TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_licenses_renewal_token
    ON billing_licenses (renewal_token);

COMMENT ON COLUMN billing_licenses.last_seen_at IS
    'When this licence last asked for a renewal. Set by GET /api/v1/billing/license, '
    'which the customer''s instance calls once a day IF they set VAKT_LICENSE_TOKEN. '
    'Opt-in, and the only thing transmitted is the token — no ISMS data. It is a '
    'signal, never a proof: a customer who does not opt in is simply never seen.';

COMMENT ON COLUMN billing_licenses.revoked_at IS
    'Stops renewals for THIS key only. It is not a kill switch — a signed key stays '
    'valid until it expires (35 or 395 days), because a self-hosted instance cannot '
    'be reached. Revocation is a closing door, not an off switch.';

-- The subscription-level token is gone: it was the bug.
DROP INDEX IF EXISTS idx_billing_quote_requests_renewal_token;
ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS renewal_token;

-- ── MSP self-service portal ──────────────────────────────────────────────────
--
-- An MSP should not have to mail Stefan every time it onboards a client. It gets a
-- link: see the seats, name the new client, get the key.
--
-- The token is stored ONLY as a SHA-256 hash — a leaked database backup must not
-- hand anyone the portal. And what a stolen portal token can actually do is bounded
-- by design: it can burn the seats the MSP already PAID for, and nothing else. It
-- cannot mint an eleventh key for ten paid seats, cannot touch another subscription,
-- and every key it issues is mailed to the MSP's registered address — so they notice.
ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS portal_token_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_billing_quote_requests_portal_token
    ON billing_quote_requests (portal_token_hash)
    WHERE portal_token_hash IS NOT NULL;
