-- Renewal token for the direct-sale (invoice) flow, and removal of the two dead
-- payment processors.
--
-- Why the token: a customer sets VAKT_LICENSE_TOKEN in their .env and their
-- instance fetches a fresh key from api.norvikops.de once a day, so a renewal
-- needs no manual step. That endpoint read polar_subscriptions.renewal_token —
-- which means it only ever worked for customers who bought through Polar. The
-- invoice flow (migration 233) issued a key by e-mail and stored no token at all,
-- so every invoice customer would have had to paste a new key by hand after 395
-- days, while .env.example and the docs promise the opposite.
--
-- Why the drops: Polar and LemonSqueezy are gone. Every product was archived on
-- 2026-07-12; the handlers are deleted in the same commit. Verified against
-- production before dropping: polar_subscriptions 0 rows, ls_subscriptions 0 rows.
-- No customer ever bought through either.
--
-- NOT dropped: license_keys (created in migration 081 alongside the LemonSqueezy
-- tables, but it is the table every instance stores its ACTIVE key in — see
-- internal/license/handler.go). It has nothing to do with the processor.

ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS renewal_token UUID NOT NULL DEFAULT gen_random_uuid();

CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_quote_requests_renewal_token
    ON billing_quote_requests (renewal_token);

DROP TABLE IF EXISTS polar_webhook_events;
DROP TABLE IF EXISTS polar_subscriptions;
DROP TABLE IF EXISTS lemonsqueezy_webhook_events;
DROP TABLE IF EXISTS ls_revoked_subscriptions;
DROP TABLE IF EXISTS ls_subscriptions;
