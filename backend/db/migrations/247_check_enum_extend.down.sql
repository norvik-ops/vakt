-- Rueckbau auf die urspruenglichen Enums. Schlaegt fehl, falls inzwischen Zeilen mit den
-- neuen Werten existieren — das ist korrekt: ein Down, der Daten entwertet, die der CHECK
-- dann verbietet, wuerde nur eine neue Zeitbombe legen. In dem Fall die Zeilen erst
-- bereinigen.
ALTER TABLE vb_reports DROP CONSTRAINT IF EXISTS vb_reports_status_check;
ALTER TABLE vb_reports
    ADD CONSTRAINT vb_reports_status_check
    CHECK (status = ANY (ARRAY['pending', 'completed', 'failed']));

ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;
ALTER TABLE ck_evidence
    ADD CONSTRAINT ck_evidence_auto_source_type_check
    CHECK (auto_source_type = ANY (ARRAY['github', 'vaktaware', 'vaktscan',
        'ci_pipeline', 'ci_webhook', 'hr']));
