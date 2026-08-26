-- 265: `unknown` ist ein zulaessiger Schweregrad fuer Funde.
--
-- Hinweis zur Nummerierung: 263 ist bewusst frei (Codeaudit v5c). Die Nummer war
-- einer Spur dieser Runde zentral zugeteilt worden; beim Bauen stellte sich
-- heraus, dass der Fix die falsche Spalte las und gar kein Schemawechsel noetig
-- war. Die Luecke ist also kein verlorener oder vergessener Schritt — hier steht
-- sie, damit ein spaeterer Leser nicht danach sucht.
--
-- Warum: Trivy und Nuclei liefern `unknown` als regulaeren Wert — bei Trivy steht
-- er sogar in der Vorgabemenge von `--severity`, und RunTrivyScan ruft trivy ohne
-- diesen Schalter auf. Der CHECK aus Migration 007 kannte den Wert nicht, also
-- schlug das Einfuegen mit SQLSTATE 23514 fehl. Weil pgx einen Batch in einer
-- impliziten Transaktion faehrt, riss eine einzige solche Zeile den ganzen Stapel
-- mit: Der Scan galt als fehlgeschlagen und es wurde nichts gespeichert.
--
-- Warum nicht einfach auf `info` abbilden: `info` heisst „bewertet, unkritisch",
-- `unknown` heisst „noch nicht bewertet". Trivy-Funde mit UNKNOWN bekommen
-- spaeter regelmaessig HIGH oder CRITICAL. Sie als `info` zu fuehren gaebe ihnen
-- den niedrigsten Risiko-Multiplikator und wiese im Auditbericht eine Bewertung
-- aus, die nie stattgefunden hat.
--
-- Reines Erweitern der erlaubten Menge: Bestandszeilen bleiben gueltig, kein
-- Rewrite der Tabelle, keine Datenaenderung.

ALTER TABLE vb_findings DROP CONSTRAINT IF EXISTS vb_findings_severity_check;

ALTER TABLE vb_findings
    ADD CONSTRAINT vb_findings_severity_check
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info', 'unknown'));

-- Dieselbe Menge fuer die SLA-Richtlinien, sonst kann eine Organisation fuer
-- unbewertete Funde keine Frist festlegen. Ohne eigene Zeile gilt weiterhin die
-- BSI-Grundschutz-Basis von 90 Tagen (defaultSLARemediationDays) — ein Fund faellt
-- also in keinem Fall aus der Ueberwachung.

ALTER TABLE vb_sla_policies DROP CONSTRAINT IF EXISTS vb_sla_policies_severity_check;

ALTER TABLE vb_sla_policies
    ADD CONSTRAINT vb_sla_policies_severity_check
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info', 'unknown'));
