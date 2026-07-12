-- Recurring billing.
--
-- Until now the sale was a one-shot: Approve() created exactly one invoice, and a
-- guard (status != 'requested') made sure it could never create a second one for
-- the same request. That is correct for a perpetual licence and WRONG for the two
-- products actually on sale — Vakt Pro is offered monthly (299 €) and yearly
-- (2.990 €), and the /angebot form has offered both since it went live.
--
-- A customer who chose "Monatslizenz — 299 €" therefore paid for a subscription
-- and received a single 35-day key. On day 36 their Pro features went dark. No
-- second invoice was ever raised, and nobody was told. Only the absence of
-- customers kept that from happening for real.
--
-- Model: a quote request BECOMES the subscription once it is approved (it already
-- carries company, e-mail, interval, Lexware contact id, licence key and renewal
-- token — that is a subscription). Invoices move out into their own table, because
-- there are now many of them per subscription and settle() must be able to tell
-- which one a payment belongs to.

ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS product         TEXT    NOT NULL DEFAULT 'pro',
    ADD COLUMN IF NOT EXISTS quantity        INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_invoice_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancelled_at    TIMESTAMPTZ;

ALTER TABLE billing_quote_requests
    ADD CONSTRAINT billing_quote_requests_quantity_positive CHECK (quantity >= 1);

COMMENT ON COLUMN billing_quote_requests.quantity IS
    'Seats. 1 for a direct customer; an MSP buys N. It is an ENTITLEMENT, not an '
    'enforcement: a licence key carries the end customer''s org name, so N seats '
    'means N different keys, issued one at a time as the MSP onboards clients. '
    'Nothing in a self-hosted instance can count activations — that would need '
    'phone-home, which this product does not do.';

COMMENT ON COLUMN billing_quote_requests.next_invoice_at IS
    'When the next invoice is due. Set ONLY when the previous one is paid — an '
    'unpaid customer must never be sent a second invoice, and a cancelled one '
    'never again.';

CREATE TABLE IF NOT EXISTS billing_invoices (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id    UUID        NOT NULL REFERENCES billing_quote_requests (id) ON DELETE CASCADE,
    lexware_invoice_id TEXT        NOT NULL UNIQUE,
    period_start       DATE        NOT NULL,
    period_end         DATE        NOT NULL,
    net_amount_cents   BIGINT      NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'open',   -- open | paid
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_billing_invoices_subscription
    ON billing_invoices (subscription_id);

-- The renewal sweep reads exactly this: subscriptions that are due and alive.
CREATE INDEX IF NOT EXISTS idx_billing_quote_requests_due
    ON billing_quote_requests (next_invoice_at)
    WHERE cancelled_at IS NULL;

-- Backfill: every invoice that already exists moves into the new table, so the
-- payment webhook keeps working for invoices that are already out in the world.
-- Amounts are reconstructed from the interval — the only two prices that have
-- ever been charged.
INSERT INTO billing_invoices
    (subscription_id, lexware_invoice_id, period_start, period_end, net_amount_cents, status, created_at, paid_at)
SELECT id,
       lexware_invoice_id,
       COALESCE(approved_at, created_at)::date,
       COALESCE(approved_at, created_at)::date
           + (CASE WHEN interval = 'year' THEN 365 ELSE 30 END),
       CASE WHEN interval = 'year' THEN 299000 ELSE 29900 END,
       CASE WHEN status = 'paid' THEN 'paid' ELSE 'open' END,
       COALESCE(approved_at, created_at),
       paid_at
  FROM billing_quote_requests
 WHERE lexware_invoice_id IS NOT NULL
ON CONFLICT (lexware_invoice_id) DO NOTHING;

-- Subscriptions that are already paid get their renewal date, so nobody who
-- bought before this migration silently falls out of the cycle.
UPDATE billing_quote_requests s
   SET next_invoice_at = (
           SELECT bi.period_end - (CASE WHEN s.interval = 'year' THEN 21 ELSE 7 END)
             FROM billing_invoices bi
            WHERE bi.subscription_id = s.id
            ORDER BY bi.period_end DESC
            LIMIT 1)
 WHERE s.status = 'paid'
   AND s.next_invoice_at IS NULL;
