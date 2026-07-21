-- SIEM-Forward-Tracking auf audit_log (S131, D389/D390, R-H10).
--
-- Der SIEM-Forward-Worker (internal/services/siem/service.go) liest
--     SELECT ... FROM audit_log WHERE forwarded_to_siem IS NULL ...
-- und markiert versendete Zeilen mit
--     UPDATE audit_log SET forwarded_to_siem = $1 WHERE id = $2.
-- Die Spalte existierte nie (42703) → der SELECT scheiterte bei jedem Lauf, der
-- Worker forwardete NIE eine einzige Zeile (born-broken).
--
-- audit_log ist partitioniert (RANGE) und hash-chained (prev_hash/entry_hash). Ein
-- ADD COLUMN auf der Partition-Parent-Tabelle propagiert auf alle Partitionen. Die
-- Spalte ist reines Forward-Metadatum, NULL bei Insert, nachträglich gesetzt — sie
-- geht nicht in die Hash-Kette ein (die deckt den Eintragsinhalt ab, nicht diesen
-- Zustellungsvermerk) und lässt bestehende Hashes unberührt. Es gibt keinen
-- Immutability-Trigger auf audit_log, der den UPDATE blockieren würde.
ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS forwarded_to_siem TIMESTAMPTZ;

-- Der Worker filtert auf forwarded_to_siem IS NULL über potenziell viele Zeilen;
-- ein partieller Index hält genau die noch offenen Einträge klein.
CREATE INDEX IF NOT EXISTS idx_audit_log_siem_pending
    ON audit_log (org_id, created_at)
    WHERE forwarded_to_siem IS NULL;

COMMENT ON COLUMN audit_log.forwarded_to_siem IS
    'Zeitpunkt der erfolgreichen SIEM-Zustellung; NULL = noch nicht forwardet. Reines '
    'Zustellungs-Metadatum, nicht Teil der Hash-Kette.';
