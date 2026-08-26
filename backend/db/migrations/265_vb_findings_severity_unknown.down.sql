-- Zurueck auf die enge Menge aus Migration 007.
--
-- Die Reihenfolge ist zwingend: Erst die Bestandszeilen umschreiben, dann den
-- engeren CHECK setzen. Andersherum schlaegt die Migration fehl, sobald auch nur
-- ein Fund mit `unknown` existiert — und nach einem Trivy-Scan existiert er.
--
-- `unknown` wird dabei zu `info`. Das ist ein Informationsverlust und genau der
-- Grund, warum diese Richtung nicht der Normalfall ist: Danach steht an einem
-- unbewerteten Fund eine Bewertung. Wer die Migration zurueckdreht, sollte das
-- wissen.

UPDATE vb_findings SET severity = 'info' WHERE severity = 'unknown';

ALTER TABLE vb_findings DROP CONSTRAINT IF EXISTS vb_findings_severity_check;

ALTER TABLE vb_findings
    ADD CONSTRAINT vb_findings_severity_check
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info'));

-- Eine Richtlinienzeile fuer `unknown` wird geloescht, nicht umgeschrieben: Sie
-- auf `info` umzubiegen wuerde eine bestehende info-Zeile ueberschreiben.

DELETE FROM vb_sla_policies WHERE severity = 'unknown';

ALTER TABLE vb_sla_policies DROP CONSTRAINT IF EXISTS vb_sla_policies_severity_check;

ALTER TABLE vb_sla_policies
    ADD CONSTRAINT vb_sla_policies_severity_check
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info'));
