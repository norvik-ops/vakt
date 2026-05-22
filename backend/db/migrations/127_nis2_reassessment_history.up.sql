-- Sprint 28 / S28-3: NIS2-Re-Assessment-History für eingeloggte Orgs.
--
-- Assessment-Runs werden hier als separate Datensätze gespeichert, damit
-- der Trend über mehrere Bewertungen hinweg sichtbar ist. Jede Org kann
-- alle 90 Tage einen neuen Run anlegen (Cooldown-Check im Service).
--
-- Abgrenzung zu ck_nis2_assessments: ck_nis2_assessments ist das
-- Einzel-Assessment nach Sign-up (Migration 125). ck_nis2_assessment_runs
-- ist die History-Tabelle für Re-Assessments (mehrere Runs pro Org).

CREATE TABLE IF NOT EXISTS ck_nis2_assessment_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    run_number      INT NOT NULL DEFAULT 1,
    answers         JSONB NOT NULL DEFAULT '{}',
    overall_score   INT,
    score_by_area   JSONB,
    top_gaps        JSONB,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON ck_nis2_assessment_runs (org_id, created_at DESC);
