-- 120: Partial UNIQUE-Indexe für die vb_findings-Dedup-Schlüssel.
--
-- Hintergrund: Die Import-/Scanner-Flows (Repository.UpsertFindingByRawID,
-- Repository.BatchUpsertFindings) nutzen ON CONFLICT-Klauseln gegen drei
-- Schlüssel-Kombinationen — die zugehörigen UNIQUE-Constraints fehlten aber
-- in Migration 007. Damit liefen die ON CONFLICT-Statements zur Laufzeit auf
-- einen Postgres-Fehler („there is no unique or exclusion constraint matching
-- the ON CONFLICT specification"), sobald sie tatsächlich aufgerufen wurden.
--
-- Diese Migration räumt erst eventuelle Duplikate auf (per CTE — neueste
-- Zeile pro Schlüssel behalten) und legt dann die partiellen UNIQUE-Indexe
-- an. „Partial" weil die jeweiligen Spalten NULL sein dürfen (cve_id /
-- template_id / raw_id) und mehrere NULL-Werte erlaubt sein müssen.

-- ── 1) Dedup für (org_id, asset_id, cve_id) WHERE cve_id IS NOT NULL ─────────
-- Bei mehrfachen Findings mit gleicher CVE auf gleichem Asset: neueste
-- (höchstes updated_at) behalten, ältere löschen. Wir nehmen updated_at, da
-- last_seen_at ggf. durch den Bug nicht aktualisiert wurde.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY org_id, asset_id, cve_id
               ORDER BY updated_at DESC, created_at DESC, id DESC
           ) AS rn
    FROM vb_findings
    WHERE cve_id IS NOT NULL
)
DELETE FROM vb_findings
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vb_findings_dedup_cve
    ON vb_findings(org_id, asset_id, cve_id)
    WHERE cve_id IS NOT NULL;

-- ── 2) Dedup für (org_id, asset_id, scanner, template_id) WHERE template_id IS NOT NULL ─
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY org_id, asset_id, scanner, template_id
               ORDER BY updated_at DESC, created_at DESC, id DESC
           ) AS rn
    FROM vb_findings
    WHERE template_id IS NOT NULL
)
DELETE FROM vb_findings
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vb_findings_dedup_template
    ON vb_findings(org_id, asset_id, scanner, template_id)
    WHERE template_id IS NOT NULL;

-- ── 3) Dedup für (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL ─────────
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY org_id, raw_id, scanner
               ORDER BY updated_at DESC, created_at DESC, id DESC
           ) AS rn
    FROM vb_findings
    WHERE raw_id IS NOT NULL
)
DELETE FROM vb_findings
WHERE id IN (SELECT id FROM ranked WHERE rn > 1);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vb_findings_dedup_rawid
    ON vb_findings(org_id, raw_id, scanner)
    WHERE raw_id IS NOT NULL;
