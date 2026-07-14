-- Vakt Aware: der Tracking-Token muss existieren, BEVOR jemand klicken kann. (Sprint 126)
--
-- Bisher konnte er das nicht, und deshalb hat Vakt Aware nie etwas gemessen:
--
--   SendCampaignEmails praegt pro Zielperson einen Token, baut ihn in den Link und
--   verschickt die Mail — und schreibt ihn NIRGENDWO hin.
--   Klickt die Zielperson, ruft RecordEvent GetCampaignByTrackingToken auf. Diese
--   Query loest den Token so auf:
--
--       FROM sr_campaigns c JOIN sr_events e ON e.campaign_id = c.id
--       WHERE e.tracking_token = $1
--
--   Der Token laesst sich also nur aufloesen, wenn zu ihm schon ein Event existiert.
--   Das einzige, was Events schreibt, ist RecordEvent/RecordOpen — die genau diese
--   Aufloesung vorher brauchen. Henne und Ei: der erste Klick einer Kampagne kann
--   per Konstruktion nie ankommen. Jeder Klick, jede Oeffnung: „invalid tracking token".
--
-- Aufgefallen ist es nie, weil der Demo-Seed sr_events direkt befuellt — in der Demo
-- sieht das Feature lebendig aus. In einer echten Instanz meldet jede Kampagne
-- strukturell 0 % Klickrate. Das ist kein fehlender Wert, sondern ein falscher: er ist
-- von „niemand ist auf die Phishing-Mail hereingefallen" nicht zu unterscheiden, fliesst
-- als Evidenz nach Vakt Comply (ISO 27001 A.6.3, NIS2 Art. 21(2)(g)) und behauptet dort
-- etwas Unwahres — plausibel unwahr, was schlimmer ist als offensichtlich unwahr.
--
-- Dies ist der dritte strukturelle Bruch derselben Funktion (S127: alle Tracking-Routen
-- hingen hinter Auth und gaben dem Empfaenger ohne Token 401; S127-2: der Klick-Link
-- zeigte auf den Open-Pixel-Pfad). Die ersten beiden Fixes waren noetig und beide
-- zusammen nicht hinreichend.
--
-- Der Fix braucht einen Event-Typ fuer „Mail rausgegangen": SendCampaignEmails schreibt
-- ihn pro Empfaenger mit dem Token, bevor die Mail die Maschine verlaesst (vorher, nicht
-- nachher — sonst gewinnt ein schneller Klick das Rennen gegen den DB-Write). Damit
-- loest der Token auf, und emails_sent ist zum ersten Mal ueberhaupt eine echte Zahl:
-- CampaignStats.EmailsSent wurde nirgends im Code je gesetzt.
ALTER TABLE sr_events DROP CONSTRAINT IF EXISTS pg_events_type_check;

ALTER TABLE sr_events ADD CONSTRAINT pg_events_type_check
    CHECK (type IN ('sent', 'open', 'click', 'form_submission'));

COMMENT ON COLUMN sr_events.type IS
    'sent = Mail an diese Zielperson rausgegangen (traegt den Tracking-Token, damit '
    'ein spaeterer Klick/Open ihn aufloesen kann). open/click/form_submission = die '
    'Reaktion der Zielperson. Ohne den sent-Eintrag ist der Token nirgends bekannt und '
    'jede Reaktion wird als "invalid tracking token" verworfen (Sprint 126).';
