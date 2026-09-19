-- Vakt Aware: auto-enrollte Mitarbeiter müssen eine Aussendung tatsächlich
-- empfangen können (ADR-0088, Befund R1-36a-D08).
--
-- Bisher trägt sr_campaign_enrollments nur employee_id (eine HR-ID). Der
-- Versandpfad (SendCampaignEmails → ListTargets → sr_targets) kennt diese Zeilen
-- nicht, und vaktaware DARF hr_employees nicht lesen (Modul-Isolation §4,
-- ADR-0079). Ohne Mail/Name in der Enrollment-Zeile ist der auto-enrollte
-- Empfänger zur Sendezeit nicht adressierbar.
--
-- Beide Spalten sind additiv und nullable: bestehende Zeilen bleiben lesbar
-- (NULL = unbekannt/vor der Migration angelegt). Kein Index, keine volatilen
-- Funktionen. Die PII fällt unter denselben EraseSubjectPII-Löscher, der die
-- Zeile ohnehin per employee_id löscht (erasure.go).
ALTER TABLE sr_campaign_enrollments
    ADD COLUMN email     TEXT,
    ADD COLUMN full_name TEXT;

COMMENT ON COLUMN sr_campaign_enrollments.email IS
    'Mail des auto-enrollten Mitarbeiters, vom HR-Producer über das Eintritts-Event '
    'geliefert (nie per Cross-Modul-Read aus hr_employees). NULL = vor ADR-0088 '
    'angelegt oder nicht adressierbar; MaterializeEnrollmentsAsTargets überspringt NULL.';
COMMENT ON COLUMN sr_campaign_enrollments.full_name IS
    'Anzeigename des auto-enrollten Mitarbeiters, Quelle wie email. NULL zulässig.';
