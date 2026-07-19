-- Was auf der Rechnung stand: Brutto, Steuer, und unter welchem Regime sie gestellt wurde.
--
-- Bis hierhin speicherte billing_invoices genau EINEN Betrag: net_amount_cents. Unter
-- § 19 UStG war das vollstaendig — netto == brutto == das, was der Kunde ueberweist.
-- Mit der Regelbesteuerung fallen die drei Zahlen zum ersten Mal auseinander, und drei
-- Dinge fehlen dann:
--
--   1. Der Betrag, den der Kunde tatsaechlich ueberweist. Fuer den Abgleich eines
--      Kontoeingangs von Hand ist genau das die einzige Zahl, die zaehlt — und sie
--      stuende nirgends.
--
--   2. Die Zahl, die ein Mensch bei der Freigabe sieht. Der Freigabe-Button und die
--      Benachrichtigungsmails zeigen den NETTObetrag, bestaetigt wird damit aber eine
--      UNUMKEHRBARE finalisierte Rechnung ueber den Bruttobetrag. Eine finalisierte
--      Lexware-Rechnung ist ueber die API nicht zurueckzunehmen; eine falsche Zahl vor
--      einer solchen Entscheidung ist das eigentliche Risiko.
--
--   3. Unter welchem Steuerregime eine Rechnung entstanden ist. Ueber die Grenze
--      § 19 → Regelbesteuerung hinweg ist das die Information, die eine spaetere
--      Pruefung braucht — und sie ist nachtraeglich NICHT rekonstruierbar. Wer in zwei
--      Jahren wissen will, warum eine Rechnung von 2026 keine Umsatzsteuer auswies,
--      findet es nur hier.
--
-- Ausdruecklich NICHT der Grund (die Annahme stand so in ADR-0073 und in einer frueheren
-- Fassung von S130, beide inzwischen korrigiert): Es droht KEIN Bruch im Abgleich.
-- reconcile.go vergleicht keine Betraege — es matcht ueber lexware_invoice_id und leitet
-- Drift aus Existenz und Status ab. settle() vergleicht ebenfalls nichts, sondern
-- verlaesst sich auf Lexwares paymentStatus == "balanced". Diese Migration schliesst eine
-- Nachweis- und Anzeigeluecke, keinen drohenden Ausfall.
--
-- Siehe ADR-0074 (warum wir die Umsatzsteuer selbst tragen) und
-- docs/stories/s130-umsatzsteuer-dach.md (AP3).

-- Die DEFAULTs sind mit Bedacht gewaehlt und NICHT bloss Bequemlichkeit.
--
-- Sie sind fuer den 0-Euro-Fall RICHTIG (Freilizenzen, siehe free.go: netto 0, keine
-- Steuer, "vatfree") — deren INSERTs nennen die Spalten nicht und sollen es auch nicht
-- muessen. Fuer jede ECHTE Rechnung sind sie dagegen falsch, und genau das ist
-- beabsichtigt: Der CHECK weiter unten (gross = net + tax) schlaegt dann zu, weil
-- 0 != 29900 + 0. Ein vergessenes Setzen faellt damit sofort und laut auf, statt eine
-- Rechnung mit falschen Betraegen zu speichern.
--
-- Ein DEFAULT, der still fuer alle Faelle "passt", waere hier das Gefaehrlichere.
ALTER TABLE billing_invoices
    ADD COLUMN IF NOT EXISTS gross_amount_cents BIGINT       DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount_cents   BIGINT       DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_rate_pct       NUMERIC(5,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_type           TEXT         DEFAULT 'vatfree';

-- Bestandszeilen tragen den Zustand, unter dem sie WIRKLICH entstanden sind — nicht NULL.
--
-- NULL hiesse „unbekannt", und das waere falsch: Jede existierende Zeile wurde unter
-- § 19 UStG gestellt, mit taxType "vatfree" und 0 % (hart im Code, siehe client.go vor
-- S130). Das ist bekannt, nicht unbekannt. Ein NULL hier wuerde die Steueruebersicht
-- (AP5) spaeter zwingen, zwischen „keine Steuer" und „weiss nicht" zu raten — und ein
-- Feld, das immer leer ist, betaeubt jede Auswertung, die es beruehrt.
UPDATE billing_invoices
   SET gross_amount_cents = COALESCE(gross_amount_cents, net_amount_cents),
       tax_amount_cents   = COALESCE(tax_amount_cents, 0),
       tax_rate_pct       = COALESCE(tax_rate_pct, 0),
       tax_type           = COALESCE(tax_type, 'vatfree');

-- Erst nach dem Backfill NOT NULL setzen, sonst scheitert die Migration an Bestandszeilen.
ALTER TABLE billing_invoices
    ALTER COLUMN gross_amount_cents SET NOT NULL,
    ALTER COLUMN tax_amount_cents   SET NOT NULL,
    ALTER COLUMN tax_rate_pct       SET NOT NULL,
    ALTER COLUMN tax_type           SET NOT NULL;

-- Die Identitaet, die immer gelten muss. Sie faengt genau den Fehler, der unter
-- Regelbesteuerung am teuersten waere: taxType "net" bei Satz 0 — Umsatzsteuer wird
-- geschuldet, aber nicht ausgewiesen (§ 14c UStG), ohne Fehler und ohne Log.
ALTER TABLE billing_invoices
    ADD CONSTRAINT billing_invoices_amounts_consistent
    CHECK (gross_amount_cents = net_amount_cents + tax_amount_cents);

COMMENT ON COLUMN billing_invoices.gross_amount_cents IS
    'Was der Kunde ueberweist. Unter § 19 UStG identisch mit net_amount_cents; ab der '
    'Regelbesteuerung der einzige Betrag, der zu einem Kontoeingang passt.';

COMMENT ON COLUMN billing_invoices.tax_type IS
    'Lexware-taxType, unter dem die Rechnung gestellt wurde: vatfree (§ 19), net '
    '(Inland), externalService13b (EU-Ausland Reverse Charge), thirdPartyCountryService '
    '(Drittland). Historischer Nachweis — nachtraeglich nicht rekonstruierbar.';

COMMENT ON COLUMN billing_invoices.tax_rate_pct IS
    'Steuersatz der Position. Gehoert IMMER mit tax_type zusammen gesetzt: net bei 0 % '
    'ist der § 14c-Fall (geschuldet, nicht ausgewiesen). Der CHECK auf die Betraege '
    'faengt ihn.';
