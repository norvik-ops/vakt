-- Rueckbau ist verlustbehaftet und das ist hier akzeptabel: Solange § 19 UStG gilt, ist
-- gross == net und tax == 0, es geht also keine Information verloren, die nicht aus
-- net_amount_cents wieder herstellbar waere.
--
-- ACHTUNG, falls dieses Down je NACH der Umstellung auf Regelbesteuerung laeuft: Dann
-- gehen Bruttobetrag, Steuerbetrag und die Angabe des Regimes UNWIEDERBRINGLICH verloren
-- — sie sind aus net_amount_cents nicht rekonstruierbar, weil der Satz je Land variiert.
-- In dem Fall vorher sichern:
--   COPY (SELECT id, gross_amount_cents, tax_amount_cents, tax_rate_pct, tax_type
--           FROM billing_invoices) TO '/tmp/billing_tax_backup.csv' CSV HEADER;

ALTER TABLE billing_invoices DROP CONSTRAINT IF EXISTS billing_invoices_amounts_consistent;

ALTER TABLE billing_invoices
    DROP COLUMN IF EXISTS gross_amount_cents,
    DROP COLUMN IF EXISTS tax_amount_cents,
    DROP COLUMN IF EXISTS tax_rate_pct,
    DROP COLUMN IF EXISTS tax_type;
