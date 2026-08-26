-- R1-19-W03, R1-20-A10, R1-06-D07: zwei Senken ohne natuerlichen Schluessel.
--
-- (1) ck_evidence
-- AddCKCollectorEvidence war ein nacktes INSERT. Der Doc-Kommentar darueber
-- behauptet "idempotently ... upsert-safe", der taegliche Cron
-- comply:bcm_evidence_sync legte damit bei jedem Lauf eine identische Zeile
-- an. Dieselbe Senke, anderer Produzent: jeder Export des
-- Awareness-Trainings-Reports schrieb zehn identische Zeilen — dort loest es
-- eine Nutzeraktion aus, nicht ein Cron.
--
-- Der natuerliche Schluessel einer maschinell gesammelten Evidenz ist
-- (org_id, control_id, source, title): derselbe Sammler, dasselbe Control,
-- dieselbe Aussage. Was sich zwischen zwei Laeufen aendert, sind die
-- Nutzdaten — die gehoeren aufgefrischt, nicht vervielfacht.
--
-- Der Index klammert bewusst aus:
--   · source = 'manual' — ein Mensch darf zwei Dateien mit demselben Titel an
--     dasselbe Control haengen.
--   · control_id IS NULL — Evidenz ohne Control (CI-Webhook vor der Zuordnung)
--     hat keinen Schluessel, an dem sich Gleichheit festmachen liesse.
--   · auto_source_type IS NOT NULL — die CI-/Integrations-Familie fuehrt mit
--     auto_source_ref eine eigene Laufkennung; zwei Laeufe mit gleichem Titel
--     sind dort zwei Tatsachen, keine doppelte.
--
-- (2) ck_control_measures
-- SeedCKMeasure benutzte ON CONFLICT DO NOTHING ohne Arbiter auf einer
-- Tabelle ohne jeden Unique-Constraint. Ohne Arbiter gibt es keinen Konflikt,
-- also greift DO NOTHING nie: die Tabelle wuchs bei JEDEM API-Start um 23
-- Zeilen (gemessen 0->23->46->69->92->115), nach fuenf Neustarts sah der Kunde
-- jede Massnahme fuenffach.
--
-- Die Variantensuche ueber alle arbiterlosen ON-CONFLICT-Stellen in
-- db/queries/vaktcomply.sql ergibt vier Fundstellen; drei sind gedeckt
-- (ck_supplier_risks und ck_risk_control_links durch einen Composite-PK,
-- ck_framework_control_mappings durch einen UNIQUE-Constraint), exakt diese
-- eine nicht.
--
-- Der Index deckt nur die eingebauten Massnahmen (is_builtin) — eine selbst
-- angelegte Massnahme darf denselben Titel tragen wie eine eingebaute.

-- ── Bestand entdoppeln, aeltester Eintrag gewinnt ───────────────────────────
DELETE FROM ck_evidence e
 USING ck_evidence keep
 WHERE e.org_id     = keep.org_id
   AND e.control_id = keep.control_id
   AND e.source     = keep.source
   AND e.title      = keep.title
   AND e.control_id IS NOT NULL
   AND e.source <> 'manual'
   AND e.auto_source_type IS NULL
   AND keep.auto_source_type IS NULL
   AND (e.created_at, e.id) > (keep.created_at, keep.id);

DELETE FROM ck_control_measures m
 USING ck_control_measures keep
 WHERE m.control_id = keep.control_id
   AND m.org_id     = keep.org_id
   AND m.title      = keep.title
   AND m.is_builtin
   AND keep.is_builtin
   AND (m.created_at, m.id) > (keep.created_at, keep.id);

-- ── Schluessel ──────────────────────────────────────────────────────────────
CREATE UNIQUE INDEX IF NOT EXISTS ck_evidence_collector_uniq
    ON ck_evidence (org_id, control_id, source, title)
 WHERE control_id IS NOT NULL
   AND source <> 'manual'
   AND auto_source_type IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ck_control_measures_builtin_uniq
    ON ck_control_measures (org_id, control_id, title)
 WHERE is_builtin;
