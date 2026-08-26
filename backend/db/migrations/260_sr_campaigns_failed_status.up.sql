-- Vakt Aware: eine Kampagne braucht einen Fehlerzustand. (L2-03)
--
-- Die einzigen Statuswerte, die das Modul je geschrieben hat, sind 'running'
-- (Start), 'aborted' (manueller Abbruch) und 'completed' (erfolgreicher Versand).
-- Bricht der Versand vorher ab — keine Vorlage, keine Zielgruppe, Vorlage nicht
-- parsebar, Zielliste nicht lesbar —, bleibt die Kampagne fuer immer auf
-- 'running'. Der Nutzer sieht keinen Fehler; der Task wird von Asynq 25-mal
-- wiederholt und dann still archiviert. Gemessen: Kampagne ohne Vorlage
-- gestartet, Task scheitert an 'campaign has no template', Status noch Minuten
-- spaeter 'running'.
--
-- 'failed' heisst: dieser Lauf ist abgebrochen, es liegt an der Kampagne oder an
-- der Umgebung, und niemand wartet mehr auf etwas. Der Unterschied zu 'aborted'
-- ist die Ursache — 'aborted' hat ein Mensch ausgeloest.

ALTER TABLE sr_campaigns DROP CONSTRAINT IF EXISTS pg_campaigns_status_check;

ALTER TABLE sr_campaigns ADD CONSTRAINT pg_campaigns_status_check
    CHECK (status IN ('draft', 'scheduled', 'running', 'completed', 'aborted', 'failed'));

COMMENT ON COLUMN sr_campaigns.status IS
    'draft/scheduled = noch nicht gestartet. running = Versand laeuft. completed = '
    'Versand durch. failed = Versand abgebrochen (Vorlage, Zielgruppe oder Mailserver). '
    'aborted = von Hand gestoppt.';
