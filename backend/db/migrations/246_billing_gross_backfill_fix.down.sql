-- Rueckbau ist rein schematisch und verlustfrei bezogen auf die Struktur: der Netto-Effekt
-- von 246 auf das Schema ist identisch mit dem Endzustand von 244 (derselbe CHECK). Der
-- Daten-Effekt (korrigiertes gross) laesst sich nicht sinnvoll rueckgaengig machen und soll
-- es auch nicht — ein falsches gross wiederherzustellen waere die Zeitbombe zurueckzubauen.
--
-- Down setzt den CHECK deshalb nur zurueck und wieder auf, damit up/down sauber paaren, ohne
-- die reparierten Betraege anzutasten.
ALTER TABLE billing_invoices
    DROP CONSTRAINT IF EXISTS billing_invoices_amounts_consistent;

ALTER TABLE billing_invoices
    ADD CONSTRAINT billing_invoices_amounts_consistent
    CHECK (gross_amount_cents = net_amount_cents + tax_amount_cents);
