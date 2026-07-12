-- Notizen und Zahlungserinnerungen.
--
-- notes: Freitext pro Kunde. Klingt nach Komfort, ist aber der Ort, an dem sonst
-- alles landet, was nirgends hinpasst — "will Rechnung per Post", "USt-ID kommt
-- nach", "Ansprechpartner wechselt zum 1.9." — und heute steht das in keiner
-- Datenbank, sondern in Stefans Kopf.
ALTER TABLE billing_quote_requests
    ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '';

-- reminded_at: wann zuletzt an eine offene Rechnung erinnert wurde. Ohne das
-- Datum schickt man entweder gar keine Erinnerung oder drei am selben Tag.
ALTER TABLE billing_invoices
    ADD COLUMN IF NOT EXISTS reminded_at TIMESTAMPTZ;

COMMENT ON COLUMN billing_invoices.reminded_at IS
    'Letzte Zahlungserinnerung. NULL = noch keine. Verhindert, dass derselbe Kunde '
    'dreimal am Tag gemahnt wird, weil jemand zweimal auf den Knopf gedrueckt hat.';
