-- Reparatur der Zeitbombe aus Migration 244 (S131-0, R-C02).
--
-- Migration 244 legte gross_amount_cents mit `ADD COLUMN ... DEFAULT 0` an und wollte
-- die Bestandszeilen danach per
--     UPDATE ... SET gross_amount_cents = COALESCE(gross_amount_cents, net_amount_cents)
-- auffuellen. Das war wirkungslos: `ADD COLUMN ... DEFAULT 0` materialisiert die Spalte
-- fuer jede bestehende Zeile mit dem Wert 0 (nicht NULL). COALESCE(0, net) ist 0 — der
-- Backfill liess gross also auf 0 stehen. Der anschliessende
--     ADD CONSTRAINT billing_invoices_amounts_consistent CHECK (gross = net + tax)
-- bricht damit bei JEDER populierten Tabelle mit net != 0 sofort mit 23514 (check_violation),
-- laesst schema_migrations dirty zurueck und macht den Verkaufsweg tot.
--
-- Auf Prod hat 244 nur deshalb nicht gefeuert, weil billing_invoices dort 0 Zeilen hat
-- (Live-lesend geprueft, SA-29/O-100). Das Fenster steht offen bis zur ersten echten
-- Rechnung: entweder eine populierte Instanz migriert 244, oder ein INSERT ohne explizites
-- gross verlaesst sich auf DEFAULT 0 und verletzt den CHECK. Diese Migration schliesst
-- beide Faelle, bevor die erste echte Rechnung entsteht.
--
-- Idempotent: laeuft korrekt, egal ob 244 den CHECK schon gesetzt hat oder an ihm
-- gescheitert ist, und egal ob gross faelschlich 0 oder bereits korrekt ist.

-- 1. CHECK loesen, damit der Repair-UPDATE nicht am alten (falschen) Zustand scheitert.
ALTER TABLE billing_invoices
    DROP CONSTRAINT IF EXISTS billing_invoices_amounts_consistent;

-- 2. gross aus der Identitaet net + tax neu berechnen. Fuer Bestandszeilen unter § 19 UStG
--    (tax = 0) ergibt das gross = net — genau das, was 244 wollte. Fuer jede spaeter unter
--    Regelbesteuerung entstandene Zeile ergibt es den tatsaechlichen Bruttobetrag.
UPDATE billing_invoices
   SET gross_amount_cents = net_amount_cents + tax_amount_cents
 WHERE gross_amount_cents <> net_amount_cents + tax_amount_cents;

-- 3. CHECK wieder herstellen — jetzt haelt er, weil die Daten stimmen.
ALTER TABLE billing_invoices
    ADD CONSTRAINT billing_invoices_amounts_consistent
    CHECK (gross_amount_cents = net_amount_cents + tax_amount_cents);
