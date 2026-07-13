-- Freilizenz-Abos werden mit entfernt: Ohne is_free waeren sie nicht mehr von
-- zahlenden zu unterscheiden, und ihre synthetischen 0-Euro-Belege („free:…") wuerden
-- in Reconcile() als erfundene Rechnungen auftauchen.
DELETE FROM billing_invoices
 WHERE subscription_id IN (SELECT id FROM billing_quote_requests WHERE is_free);
DELETE FROM billing_quote_requests WHERE is_free;

ALTER TABLE billing_quote_requests DROP COLUMN IF EXISTS is_free;
