-- R1-14b-A1 (zweiter Teil): ck_controls.evidence_status hatte genau einen
-- Schreiber — den naechtlichen 03:30-Cron (Repository.UpdateEvidenceStaleness).
--
-- GET /vaktcomply/compliance-score aggregiert ausschliesslich diese Spalte.
-- Ein Nachweis, den ein Nutzer morgens hochlaedt, bewegte den Wert deshalb
-- bis zum naechsten Nachtlauf um nichts: eine Organisation mit 27 Controls
-- MIT Nachweis meldete den ganzen Tag "0 % erfuellt".
--
-- Die Nachweise entstehen an vielen Stellen — manueller Upload, rund 30
-- Collector-Aufrufstellen, Demo-Seed, Trainings-Report. Jede einzelne um
-- einen Neuberechnungs-Aufruf zu ergaenzen, verschiebt den Defekt nur auf
-- die naechste Stelle, die es vergisst. Die Neuberechnung sitzt deshalb als
-- Trigger auf ck_evidence und deckt jeden Produzenten ab, auch kuenftige.
--
-- Der Cron bleibt: er ist weiterhin noetig, weil 'stale' vom Zeitablauf
-- abhaengt (evidence_max_age_days), nicht von einer Aenderung an ck_evidence.
-- Der Trigger deckt die ereignisgetriebene Haelfte ab, der Cron die
-- zeitgetriebene. Die CASE-Logik ist bewusst identisch zu
-- Repository.UpdateEvidenceStaleness.

CREATE OR REPLACE FUNCTION ck_refresh_evidence_status(p_control_id UUID)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE ck_controls c
       SET evidence_status = CASE
               WHEN c.not_applicable = true THEN 'na'
               WHEN e.newest IS NULL THEN 'missing'
               WHEN c.evidence_max_age_days IS NULL THEN 'ok'
               WHEN NOW() - e.newest > (c.evidence_max_age_days * INTERVAL '1 day') THEN 'stale'
               ELSE 'ok'
           END,
           evidence_last_updated = e.newest,
           evidence_expires_at = CASE
               WHEN c.evidence_max_age_days IS NOT NULL AND e.newest IS NOT NULL
               THEN e.newest + (c.evidence_max_age_days * INTERVAL '1 day')
               ELSE NULL
           END
      FROM (
          SELECT MAX(created_at) AS newest
            FROM ck_evidence
           WHERE control_id = p_control_id
      ) e
     WHERE c.id = p_control_id;
$$;

CREATE OR REPLACE FUNCTION ck_evidence_status_trigger()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    -- Bei UPDATE kann der Nachweis auf ein anderes Control umgehaengt worden
    -- sein; dann muessen beide Seiten neu gerechnet werden.
    IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
        PERFORM ck_refresh_evidence_status(OLD.control_id);
    END IF;
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        PERFORM ck_refresh_evidence_status(NEW.control_id);
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_ck_evidence_status ON ck_evidence;
CREATE TRIGGER trg_ck_evidence_status
    AFTER INSERT OR UPDATE OR DELETE ON ck_evidence
    FOR EACH ROW
    EXECUTE FUNCTION ck_evidence_status_trigger();

-- Bestandsdaten einmalig nachziehen: Instanzen, die seit dem Erst-Release
-- laufen, tragen den Wert, den der letzte Cron-Lauf hinterlassen hat — auf
-- einer Instanz, deren Worker nie lief, ist das durchgehend 'missing'.
UPDATE ck_controls c
   SET evidence_status = CASE
           WHEN c.not_applicable = true THEN 'na'
           WHEN e.newest IS NULL THEN 'missing'
           WHEN c.evidence_max_age_days IS NULL THEN 'ok'
           WHEN NOW() - e.newest > (c.evidence_max_age_days * INTERVAL '1 day') THEN 'stale'
           ELSE 'ok'
       END,
       evidence_last_updated = e.newest,
       evidence_expires_at = CASE
           WHEN c.evidence_max_age_days IS NOT NULL AND e.newest IS NOT NULL
           THEN e.newest + (c.evidence_max_age_days * INTERVAL '1 day')
           ELSE NULL
       END
  FROM (
      SELECT c2.id AS control_id, MAX(ev.created_at) AS newest
        FROM ck_controls c2
        LEFT JOIN ck_evidence ev ON ev.control_id = c2.id
       GROUP BY c2.id
  ) e
 WHERE c.id = e.control_id;
