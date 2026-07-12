-- Angebotsanfragen für den Direktverkauf (Rechnung + Überweisung).
--
-- Ersetzt den Kauf über einen US-Merchant-of-Record: Vakt verkauft B2B im
-- DACH-Raum, dort ist Kauf auf Rechnung der Normalfall, und Kundendaten bei
-- einer Delaware-Corp passten schlecht zu einem Produkt, dessen Kernversprechen
-- Datensouveränität ist.
--
-- Lebt nur auf der Billing-Instanz (api.norvikops.de). Auf einer Kunden-Instanz
-- bleibt die Tabelle leer — dort ist weder VAKT_LEXWARE_API_KEY noch der
-- Signaturschlüssel gesetzt.
CREATE TABLE IF NOT EXISTS billing_quote_requests (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Aus dem öffentlichen Formular. Nichts davon ist vertrauenswürdig.
    company_name        TEXT        NOT NULL,
    contact_name        TEXT        NOT NULL DEFAULT '',
    email               TEXT        NOT NULL,
    vat_id              TEXT        NOT NULL DEFAULT '',
    street              TEXT        NOT NULL DEFAULT '',
    zip                 TEXT        NOT NULL DEFAULT '',
    city                TEXT        NOT NULL DEFAULT '',
    country_code        TEXT        NOT NULL DEFAULT 'DE',
    note                TEXT        NOT NULL DEFAULT '',
    interval            TEXT        NOT NULL DEFAULT 'year',

    -- requested -> approved (Rechnung erstellt, Trial-Key raus) -> paid (Vollkey raus)
    status              TEXT        NOT NULL DEFAULT 'requested',

    -- Nur der Hash des Freigabe-Tokens wird gespeichert. Wer die DB liest, kann
    -- damit keine Rechnung freigeben — dasselbe Prinzip wie bei Passwörtern.
    approval_token_hash TEXT        NOT NULL,

    lexware_contact_id  TEXT,
    lexware_invoice_id  TEXT,
    license_key         TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at         TIMESTAMPTZ,
    paid_at             TIMESTAMPTZ,

    CONSTRAINT billing_quote_requests_status_check
        CHECK (status IN ('requested', 'approved', 'paid', 'rejected'))
);

-- Der Zahlungs-Webhook kennt nur die Lexware-Rechnungs-ID und muss darüber die
-- Anfrage finden. Ohne Index wäre das ein Seq-Scan auf jedem Webhook.
CREATE INDEX IF NOT EXISTS idx_billing_quote_requests_invoice
    ON billing_quote_requests (lexware_invoice_id)
    WHERE lexware_invoice_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_billing_quote_requests_status
    ON billing_quote_requests (status, created_at DESC);
