ALTER TABLE billing_quote_requests
    DROP CONSTRAINT IF EXISTS billing_quote_requests_discount_range;
ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS discount_percent;

ALTER TABLE billing_invoices DROP COLUMN IF EXISTS discount_percent;
ALTER TABLE billing_invoices DROP COLUMN IF EXISTS list_amount_cents;
