ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS notes;
ALTER TABLE billing_invoices DROP COLUMN IF EXISTS reminded_at;
