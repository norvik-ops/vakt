-- The last invoice per subscription moves back into the parent row, so the
-- pre-236 settle() (which matched on billing_quote_requests.lexware_invoice_id)
-- keeps working.
UPDATE billing_quote_requests s
   SET lexware_invoice_id = (
           SELECT bi.lexware_invoice_id
             FROM billing_invoices bi
            WHERE bi.subscription_id = s.id
            ORDER BY bi.created_at DESC
            LIMIT 1)
 WHERE EXISTS (SELECT 1 FROM billing_invoices bi WHERE bi.subscription_id = s.id);

DROP TABLE IF EXISTS billing_invoices;

DROP INDEX IF EXISTS idx_billing_quote_requests_due;

ALTER TABLE billing_quote_requests
    DROP CONSTRAINT IF EXISTS billing_quote_requests_quantity_positive;

ALTER TABLE billing_quote_requests
    DROP COLUMN IF EXISTS product,
    DROP COLUMN IF EXISTS quantity,
    DROP COLUMN IF EXISTS next_invoice_at,
    DROP COLUMN IF EXISTS cancelled_at;
