-- Rabatt pro Kunde.
--
-- Bis hierhin gab es genau zwei Preise, und sie standen im Plan-Katalog. Ein
-- Nachlass fuer einen Early Adopter, einen Verband oder einen MSP war nur so zu
-- machen: die Rechnung von Hand in Lexware stellen. Damit weiss Vakt vom Verkauf
-- nichts — kein Abo, keine Verlaengerung, kein Schluessel, und in der Reconcile
-- taucht die Rechnung als "nur in Lexware" auf.
--
-- Der Rabatt haengt am ABO, nicht an der Rechnung, weil er dauerhaft gilt: er
-- muss die Verlaengerung (renewOne) ueberleben. Stuende er nur auf der ersten
-- Rechnung, zahlte der Kunde ab Periode 2 stillschweigend den vollen Preis —
-- niemand haette es gemerkt ausser dem Kunden.
ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS discount_percent SMALLINT NOT NULL DEFAULT 0;

-- Die Obergrenze ist 90, nicht 100, und das ist kein Geschmack:
--
-- 100 % waeren eine 0-Euro-Rechnung. Der Kunde ueberweist nie, Lexware meldet
-- den Beleg deshalb nie als "balanced", settle() laeuft nie — und settle() ist
-- die EINZIGE Stelle, die den Vollschluessel ausstellt und next_invoice_at
-- setzt. Das Abo waere ab Tag 45 tot, ohne eine einzige Fehlermeldung.
--
-- Gratis vergeben geht trotzdem — aber ueber einen eigenen Weg, nicht ueber einen
-- Rabatt von 100 %: die FREILIZENZ (is_free, Migration 241). Die stellt gar keine
-- Rechnung, umgeht Lexware komplett und haengt deshalb an nichts, was bezahlt
-- werden muesste.
ALTER TABLE billing_quote_requests
    ADD CONSTRAINT billing_quote_requests_discount_range
    CHECK (discount_percent >= 0 AND discount_percent <= 90);

COMMENT ON COLUMN billing_quote_requests.discount_percent IS
    'Dauerhafter Nachlass in Prozent auf den Listenpreis, gilt fuer JEDE Rechnung '
    'dieses Abos, auch fuer Verlaengerungen. 0 = Listenpreis. Max. 90: eine '
    '0-Euro-Rechnung wird nie bezahlt, also wuerde settle() nie laufen und der '
    'Kunde nie einen Vollschluessel bekommen.';

-- Und derselbe Wert noch einmal auf der Rechnung — aber als Kopie, nicht als
-- Verweis. Aendert sich der Rabatt eines Kunden spaeter, darf das die bereits
-- gestellten Rechnungen nicht rueckwirkend umschreiben: die Rechnung ist ein
-- Beleg, kein View auf den heutigen Stand. net_amount_cents ist der bezahlte
-- Nettobetrag; list_amount_cents haelt fest, wovon er abgezogen wurde.
ALTER TABLE billing_invoices
    ADD COLUMN IF NOT EXISTS discount_percent SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS list_amount_cents BIGINT;

-- Bestandsrechnungen: ohne Rabatt gestellt, also ist der Listenpreis der
-- Nettobetrag. Kein Rateschritt noetig.
UPDATE billing_invoices
   SET list_amount_cents = net_amount_cents
 WHERE list_amount_cents IS NULL;

COMMENT ON COLUMN billing_invoices.list_amount_cents IS
    'Listenpreis vor Rabatt. net_amount_cents ist das, was tatsaechlich auf der '
    'Rechnung steht. Beide werden gespeichert, weil der Rabatt eines Kunden sich '
    'aendern darf und ein Beleg nicht rueckwirkend anders lauten kann.';
