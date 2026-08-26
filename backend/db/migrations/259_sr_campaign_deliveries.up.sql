-- Vakt Aware: eine Kampagne muss wissen, wen sie schon angeschrieben hat. (L3-05, L2-01)
--
-- Bisher wusste sie es nicht. SendCampaignEmails prueft an keiner Stelle, ob die
-- Kampagne bereits versendet wurde, und kennt keine Sperre je Empfaenger. Der
-- Versand ist ein Asynq-Task; Asynq stellt einen Task erneut zu, wenn der Handler
-- einen Fehler zurueckgibt oder wenn der Worker mitten im Task stirbt (Neustart,
-- Watchtower, OOM) und die Lease ablaeuft. Gemessen: eine abgeschlossene Kampagne
-- ein zweites Mal versendet = jeder Mitarbeiter bekommt die Phishing-Mail erneut,
-- und die Kennzahl meldet danach doppelt so viele Zustellungen wie es Empfaenger
-- gibt — die Klickrate halbiert sich, sieht also besser aus als die Wirklichkeit.
--
-- Warum eine eigene Tabelle und nicht sr_events:
--
--   sr_events traegt im Betriebsrat-Modus bewusst KEIN target_id (§87 BetrVG,
--   DSGVO Art. 22) — dort ist die Zeile anonym, und genau deshalb kann sie nicht
--   beantworten, wer schon eine Mail hat. Der Wiederanlaufschutz braucht diese
--   Antwort aber in JEDEM Modus, sonst schuetzt er ausgerechnet die Kampagnen
--   nicht, die unter Mitbestimmung laufen.
--
--   Der Eintrag hier ist datenschutzrechtlich etwas anderes als ein Tracking-
--   Ereignis: er sagt „an diese Zielperson ging eine Mail" — eine Tatsache, die
--   schon aus der Gruppenmitgliedschaft folgt, weil eine Kampagne an alle in der
--   Gruppe geht. Er sagt NICHT, wer geklickt hat; der Tracking-Token steht
--   absichtlich nicht in dieser Tabelle. Die Verbindung Klick → Person bleibt
--   damit genau da, wo der Betriebsrat-Modus sie kappt.
--
-- Ablauf: der Versand beansprucht jede Zielperson mit einem INSERT ... ON CONFLICT
-- (status='pending'). Kommt keine Zeile zurueck, ist die Person fuer diesen Lauf
-- gesperrt. Nach dem Versand wird der beanspruchte Satz auf 'delivered' oder
-- 'failed' gesetzt.
--
-- Die drei Vorzustaende werden bewusst unterschiedlich behandelt:
--
--   'delivered' → gesperrt. Fertig.
--   'pending'   → gesperrt. Ein frueherer Lauf ist zwischen Anspruch und
--                 Rueckmeldung gestorben; ob die Mail draussen ist, weiss
--                 niemand. Unklar ist besser als doppelt, und der Zustand steht
--                 sichtbar in der Tabelle statt geraten zu werden.
--   'failed'    → wieder beanspruchbar. Hier ist BEKANNT, dass keine Mail
--                 rausging. Waere auch dieser Zustand gesperrt, machte die
--                 Sperre einen voruebergehenden SMTP-Ausfall endgueltig: der
--                 Wiederholungslauf faende alles beansprucht, verschickte nichts
--                 und meldete die Kampagne als abgeschlossen — mit null Mails.
--                 Das waere dieselbe falsche Null, gegen die diese Tabelle
--                 ueberhaupt angelegt wurde, nur eine Ebene spaeter.

CREATE TABLE IF NOT EXISTS sr_campaign_deliveries (
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    campaign_id UUID        NOT NULL REFERENCES sr_campaigns(id)  ON DELETE CASCADE,
    target_id   UUID        NOT NULL REFERENCES sr_targets(id)    ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'delivered', 'failed')),
    last_error  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, target_id)
);

CREATE INDEX IF NOT EXISTS idx_sr_campaign_deliveries_org
    ON sr_campaign_deliveries (org_id, campaign_id, status);

COMMENT ON TABLE sr_campaign_deliveries IS
    'Zustellbuch je Kampagne und Zielperson. Zweck: Wiederanlaufschutz (eine erneut '
    'zugestellte Asynq-Task darf niemanden zweimal anschreiben) und die Wahrheit '
    'darueber, welche Mail die Maschine wirklich verlassen hat. Traegt bewusst KEINEN '
    'Tracking-Token — die Zuordnung Klick zu Person bleibt dem Betriebsrat-Modus '
    'ueberlassen.';
