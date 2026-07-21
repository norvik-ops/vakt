-- CHECK-Enums an den Code angleichen (S131-A2-scan, C1/C2/C3).
--
-- Drei born-broken Schreibpfade verletzten je einen CHECK-Constraint mit 23514 — der
-- Code schrieb einen Wert, den der Constraint verbietet. Der Code ist jeweils korrekt
-- (die Werte sind legitime, gewollte Kategorien); das Enum war nur nie nachgezogen.
--
--   C1: cmd/worker/handlers_secpulse.go setzt vb_reports.status = 'processing' als
--       transienten Zustand → Task bricht, Report nie erstellt.
--   C2: internal/services/evidence_auto/collector.go schreibt auto_source_type
--       'github_ghas' (Dependabot/Secret/Code-Scanning) → 0 Evidence importiert.
--   C3: internal/modules/vaktcomply/hr_integration.go schreibt 'personio' → 0 Evidence.

-- vb_reports.status (NOT NULL): 'processing' ergaenzen.
ALTER TABLE vb_reports DROP CONSTRAINT IF EXISTS vb_reports_status_check;
ALTER TABLE vb_reports
    ADD CONSTRAINT vb_reports_status_check
    CHECK (status = ANY (ARRAY['pending', 'processing', 'completed', 'failed']));

-- ck_evidence.auto_source_type (nullable — NULL passiert den CHECK weiterhin):
-- 'github_ghas' und 'personio' ergaenzen.
ALTER TABLE ck_evidence DROP CONSTRAINT IF EXISTS ck_evidence_auto_source_type_check;
ALTER TABLE ck_evidence
    ADD CONSTRAINT ck_evidence_auto_source_type_check
    CHECK (auto_source_type = ANY (ARRAY['github', 'vaktaware', 'vaktscan',
        'ci_pipeline', 'ci_webhook', 'hr', 'github_ghas', 'personio']));
