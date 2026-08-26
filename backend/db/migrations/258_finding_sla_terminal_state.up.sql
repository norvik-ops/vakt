-- R1-27-V03: die als NIS2 Art. 21f ausgewiesene SLA-Einhaltungsquote war ein
-- Narben-Zaehler.
--
-- calcFindingSLACompliance (reporting/kpi_calculator.go) rechnet
--   erfuellt  = on_track | at_risk | resolved_on_time
--   verletzt  = overdue  | resolved_late
-- Die beiden Zustaende, die "rechtzeitig geloest" ausdruecken, hatte kein
-- Codepfad je geschrieben. Der taegliche SLA-Cron (RunSLACheckForOrg) laeuft
-- ueber ListOpenFindingsWithSLA, also nur ueber OFFENE Findings, und kann
-- deshalb nur on_track, at_risk oder overdue setzen. Ein Finding, das einmal
-- ueberfaellig war, blieb auch nach der Behebung fuer immer 'overdue'.
--
-- Ebenfalls aus Migration 187 und ebenfalls ohne Schreiber UND ohne Leser:
-- vb_findings.sla_resolved_within.
--
-- Warum ein Trigger und nicht ein Aufruf im Auflösungspfad: vb_findings wird
-- aus mehreren Richtungen auf 'resolved' gesetzt (Handler, Dedup-Import,
-- SBOM-Abgleich, Wiedereroeffnung durch erneuten Scan). Ein Aufruf je Pfad
-- waere genau die Bauform, die diesen Defekt erzeugt hat. Der Trigger sitzt
-- am Statuswechsel selbst.
--
-- Warum das hier und nicht in vaktcomply: die Spalte gehoert zu vb_findings,
-- also zu Vakt Scan. Ein Schreibzugriff aus vaktcomply waere genau der
-- Praefix-Bruch, den die CLAUDE.md-Invariante verbietet (siehe R1-14b-A6).
-- Eine Migration ist modulneutral.
--
-- Ein Fehlalarm (false_positive) faellt aus der Messung: er war nie eine
-- Behebungspflicht. Ihn als resolved_on_time zu zaehlen wuerde die Quote
-- schoenen, als resolved_late waere er eine erfundene Verfehlung. sla_status
-- wird deshalb auf NULL gesetzt — die Kennzahl filtert sla_status IS NOT NULL.
-- 'accepted_risk' bleibt bewusst unberuehrt: ein akzeptiertes Risiko ist noch
-- offen, nur eben bewusst, und der Cron soll es weiter bewerten.

CREATE OR REPLACE FUNCTION vb_findings_sla_terminal_state()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IS NOT DISTINCT FROM OLD.status THEN
        RETURN NEW;
    END IF;

    IF NEW.status = 'resolved' THEN
        IF NEW.sla_due_at IS NOT NULL THEN
            NEW.sla_resolved_within := (NOW() <= NEW.sla_due_at);
            NEW.sla_status := CASE
                WHEN NOW() <= NEW.sla_due_at THEN 'resolved_on_time'
                ELSE 'resolved_late'
            END;
        END IF;
    ELSIF NEW.status = 'false_positive' THEN
        NEW.sla_status := NULL;
        NEW.sla_resolved_within := NULL;
    ELSIF OLD.status IN ('resolved', 'false_positive') THEN
        -- Wiedereroeffnet: der Endzustand gilt nicht mehr. Zurueck auf den
        -- Anfangswert; der naechste Cron-Lauf bewertet neu.
        NEW.sla_resolved_within := NULL;
        IF NEW.sla_due_at IS NOT NULL THEN
            NEW.sla_status := 'on_track';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_vb_findings_sla_terminal_state ON vb_findings;
CREATE TRIGGER trg_vb_findings_sla_terminal_state
    BEFORE UPDATE ON vb_findings
    FOR EACH ROW
    EXECUTE FUNCTION vb_findings_sla_terminal_state();

-- Bestandsdaten: bereits geloeste Findings tragen noch ihren letzten offenen
-- Zustand. Bewertet wird gegen updated_at — die Aufloesungszeit selbst ist
-- nicht gespeichert, updated_at ist die beste verfuegbare Naeherung. Das
-- steht hier, damit niemand die nachgetragenen Werte fuer gemessen haelt.
UPDATE vb_findings
   SET sla_resolved_within = (updated_at <= sla_due_at),
       sla_status = CASE WHEN updated_at <= sla_due_at
                         THEN 'resolved_on_time' ELSE 'resolved_late' END
 WHERE status = 'resolved'
   AND sla_due_at IS NOT NULL
   AND sla_status NOT IN ('resolved_on_time', 'resolved_late');

UPDATE vb_findings
   SET sla_status = NULL, sla_resolved_within = NULL
 WHERE status = 'false_positive';
