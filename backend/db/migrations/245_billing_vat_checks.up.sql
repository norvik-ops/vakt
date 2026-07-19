-- Der Nachweis, dass die USt-IdNr. eines Kunden geprüft wurde — und wann.
--
-- Warum das eine eigene Tabelle ist und kein Flag auf der Bestellung:
--
--   1. Bei Reverse Charge geht die Steuerschuld auf den Kunden über, ABER nur bei
--      nachgewiesener Unternehmereigenschaft. Ist die Nummer ungültig, schulden WIR die
--      Umsatzsteuer, die wir nie berechnet haben. Der Nachweis ist damit kein Protokoll,
--      sondern das, was im Zweifel den Unterschied macht.
--
--   2. Reverse Charge verlangt eine zum Zeitpunkt DES UMSATZES gültige Nummer. Eine
--      Verlängerung ein Jahr später braucht eine EIGENE Prüfung — die alte belegt für
--      sie nichts. Also: viele Prüfungen pro Abo, nicht eine.
--
--   3. Lexware speichert das nicht. Es übermittelt die Zusammenfassende Meldung anhand
--      der am Kontakt hinterlegten Nummer, prüft sie aber nicht und führt keine Historie.
--
-- Siehe docs/stories/s130-umsatzsteuer-dach.md (AP2) und ADR-0074.

CREATE TABLE IF NOT EXISTS billing_vat_checks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID        NOT NULL REFERENCES billing_quote_requests(id) ON DELETE CASCADE,

    vat_id          TEXT        NOT NULL,
    country_code    TEXT        NOT NULL,

    valid           BOOLEAN     NOT NULL,

    -- qualified = mit Name und Anschrift abgeglichen. NUR eine qualifizierte Bestätigung
    -- trägt als Nachweis; die einfache Abfrage fängt Tippfehler und erloschene Nummern,
    -- mehr nicht. Bleibt false, solange NorvikOps keine eigene USt-IdNr. hat (die ist
    -- Voraussetzung, um qualifiziert anfragen zu dürfen) — S130, offener Punkt 5.
    qualified       BOOLEAN     NOT NULL DEFAULT false,

    trader_name     TEXT        NOT NULL DEFAULT '',
    trader_address  TEXT        NOT NULL DEFAULT '',

    -- raw_status trennt "ungültig" von "nicht prüfbar". Beide führen dazu, dass NICHT
    -- mit Reverse Charge abgerechnet wird — aber nur eines davon ist ein Problem des
    -- Kunden. Wer beides als valid=false speichert, ohne den Grund festzuhalten, kann
    -- hinterher nicht mehr unterscheiden, ob die Nummer schlecht war oder der Dienst.
    raw_status      TEXT        NOT NULL,

    -- Die Vorgangskennung, die VIES nur bei einer QUALIFIZIERTEN Anfrage vergibt. Sie
    -- ist der eigentliche Beleg gegenueber dem Finanzamt: "wir haben am Tag X unter der
    -- Kennung Y geprueft". Leer heisst geprueft, aber nicht nachweisbar geprueft — der
    -- heutige Zustand, solange NorvikOps keine eigene USt-IdNr. hat.
    request_identifier TEXT     NOT NULL DEFAULT '',

    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Die übliche Frage lautet "gibt es für dieses Abo eine gültige Prüfung, und wie alt ist
-- sie?" — also nach Abo, neueste zuerst.
CREATE INDEX IF NOT EXISTS billing_vat_checks_subscription_idx
    ON billing_vat_checks (subscription_id, checked_at DESC);

COMMENT ON TABLE billing_vat_checks IS
    'VIES-Pruefungen auslaendischer USt-IdNr. Eine Zeile je Pruefung, nicht je Kunde: '
    'Reverse Charge verlangt eine zum Zeitpunkt des Umsatzes gueltige Nummer, jede '
    'Verlaengerung braucht deshalb eine eigene Pruefung.';

COMMENT ON COLUMN billing_vat_checks.qualified IS
    'Qualifizierte Bestaetigung (mit Name/Anschrift abgeglichen) — nur diese traegt als '
    'Nachweis. Erfordert eine eigene USt-IdNr. als Anfragenden; bis dahin immer false.';
