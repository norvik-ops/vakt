-- Freilizenz: eine Lizenz ohne Rechnung.
--
-- Bis hierhin gab es keinen Weg, jemandem Vakt zu geben, ohne ihm etwas zu berechnen
-- — und zwar keinen versteckten, sondern gar keinen. Die ganze Kette haengt an einer
-- BEZAHLTEN Rechnung:
--
--   settle()          stellt den Vollschluessel aus — laeuft nur, wenn Lexware Geld meldet
--   Entitlement()     leitet die Gueltigkeit aus MAX(period_end) BEZAHLTER Rechnungen ab
--   Seats.State()     verlangt status = 'paid' auf dem Abo
--
-- „Platz vergeben" war damit ohne bezahlte Rechnung nicht erreichbar. Ein 100-%-Rabatt
-- ist ebenfalls kein Ausweg: Eine 0-Euro-Rechnung wird nie ueberwiesen, also meldet
-- Lexware sie nie als „balanced", also laeuft settle() nie — der Kunde bekaeme nie
-- einen Vollschluessel und das Abo waere an Tag 45 lautlos tot. Deshalb ist der Rabatt
-- bei 90 % gedeckelt (Migration 240) und „gratis" ein EIGENER Fall.
ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS is_free BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN billing_quote_requests.is_free IS
    'Freilizenz: Design-Partner, Referenzkunde, Verband, Beta-Tester. Es wird KEIN '
    'Lexware-Kontakt und KEINE Rechnung angelegt — der Schluessel wird direkt '
    'ausgestellt. Nur im Panel setzbar, nie ueber das oeffentliche Formular.';

-- Und hier steckt der eigentliche Kniff.
--
-- Die Periode einer Freilizenz wird als BEZAHLTE 0-Euro-Rechnung verbucht — mit einer
-- synthetischen Belegnummer (`free:<abo>:<datum>`) statt einer aus Lexware. Damit
-- braucht KEIN Stueck der Lizenz-Maschinerie eine Sonderbehandlung: Entitlement,
-- Schluessel-Erneuerung, Auto-Renewal ueber VAKT_LICENSE_TOKEN und „Platz vergeben"
-- lesen weiterhin genau das, was sie immer gelesen haben — bezahlte Perioden. Die
-- Alternative waere gewesen, an jeder dieser Stellen ein „oder gratis" einzubauen,
-- und eine davon haette man vergessen.
--
-- Der Preis dafuer: Diese Zeilen haben kein Gegenstueck in Lexware. Reconcile() haette
-- sie deshalb als „nur in Vakt" gemeldet — den SCHWERWIEGENDEN Fall, der eigentlich
-- „wir haben eine Rechnung erfunden" bedeutet. Reconcile() schliesst Freilizenzen
-- darum ausdruecklich aus (siehe reconcile.go).
COMMENT ON COLUMN billing_invoices.lexware_invoice_id IS
    'Belegnummer aus Lexware. Bei einer Freilizenz steht hier stattdessen '
    '`free:<abo-id>:<periodenstart>` — ein synthetischer Wert, der in Lexware NICHT '
    'existiert. Reconcile() ueberspringt diese Zeilen, sonst meldete es sie als '
    'erfundene Rechnung.';

-- Bewusst KEIN CHECK-Constraint auf „gratis hat keinen Lexware-Kontakt": Ein Kunde
-- darf von gratis auf zahlend wechseln (Design-Partner wird Kunde), und dann traegt
-- dasselbe Abo beides. Ein Constraint, der das verbietet, wuerde genau den Uebergang
-- blockieren, fuer den es Freilizenzen ueberhaupt gibt.
