-- Vakt Scan queries — sqlc migration in v0.6.x (ADR-0005).
--
-- Migrationsstand:
--   ✅ vb_assets     — sqlc (this file)
--   ✅ vb_sla_config — sqlc
--   ✅ vb_scans      — sqlc
--   ⏳ vb_findings   — embedded SQL (Sitzung C2 — Dedup-Logik)
--   ⏳ vb_components / vb_sboms / vb_reports / vb_finding_suppressions /
--      vb_scan_schedules / vb_eol_cache — embedded SQL (Sitzung C3)

-- ── Assets ──────────────────────────────────────────────────────────────────

-- name: CreateSPAsset :one
INSERT INTO vb_assets (org_id, name, type, criticality, tags, owner_id, external_url)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, name, type, criticality, tags, owner_id, external_url,
          created_at, updated_at;

-- name: CountSPAssets :one
SELECT COUNT(*) FROM vb_assets
WHERE org_id = $1 AND is_deleted = FALSE
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(tags));

-- name: ListSPAssets :many
SELECT id, org_id, name, type, criticality, tags, owner_id, external_url,
       created_at, updated_at
FROM vb_assets
WHERE org_id = $1 AND is_deleted = FALSE
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(tags))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetSPAsset :one
SELECT id, org_id, name, type, criticality, tags, owner_id, external_url,
       created_at, updated_at
FROM vb_assets
WHERE id = $1 AND org_id = $2 AND is_deleted = FALSE;

-- name: GetSPAssetByName :one
SELECT id, org_id, name, type, criticality, tags, owner_id, external_url,
       created_at, updated_at
FROM vb_assets
WHERE org_id = $1 AND lower(name) = lower($2::text) AND is_deleted = FALSE
LIMIT 1;

-- name: UpdateSPAsset :one
-- Updates all fields. Callers must read-merge-write (load current asset, apply
-- partial updates from UpdateAssetInput, then pass the merged values here).
-- The dynamic SET-clause builder in the previous embedded SQL is not portable
-- to sqlc; the read-merge-write trade-off is one extra round-trip in exchange
-- for type-safe, generated code.
UPDATE vb_assets SET
  name         = $3,
  type         = $4,
  criticality  = $5,
  tags         = $6,
  owner_id     = $7,
  external_url = $8,
  updated_at   = NOW()
WHERE id = $1 AND org_id = $2 AND is_deleted = FALSE
RETURNING id, org_id, name, type, criticality, tags, owner_id, external_url,
          created_at, updated_at;

-- name: SoftDeleteSPAsset :execrows
UPDATE vb_assets SET is_deleted = TRUE, updated_at = NOW()
WHERE id = $1 AND org_id = $2 AND is_deleted = FALSE;

-- ── SLA-Config ──────────────────────────────────────────────────────────────

-- name: GetSPSLAConfig :one
SELECT org_id, critical_days, high_days, medium_days, low_days
FROM vb_sla_config
WHERE org_id = $1;

-- name: UpsertSPSLAConfig :exec
INSERT INTO vb_sla_config (org_id, critical_days, high_days, medium_days, low_days, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (org_id) DO UPDATE
  SET critical_days = EXCLUDED.critical_days,
      high_days     = EXCLUDED.high_days,
      medium_days   = EXCLUDED.medium_days,
      low_days      = EXCLUDED.low_days,
      updated_at    = NOW();

-- name: GetSPSLADashboard :many
SELECT
    a.id           AS asset_id,
    a.name         AS asset_name,
    f.id           AS finding_id,
    f.title        AS finding_title,
    f.severity,
    f.status,
    EXTRACT(DAY FROM now() - f.created_at)::int AS days_open
FROM vb_findings f
JOIN vb_assets a ON a.id = f.asset_id
WHERE f.org_id = $1
  AND f.status NOT IN ('resolved', 'false_positive')
ORDER BY f.severity DESC, days_open DESC
LIMIT 100;

-- ── Scans ───────────────────────────────────────────────────────────────────

-- name: CreateSPScan :one
INSERT INTO vb_scans (org_id, asset_id, scanner, status, target_url, target_ip)
VALUES ($1, $2, $3, 'pending', $4, $5)
RETURNING id, org_id, asset_id, scanner, status,
          target_url, target_ip, error_message, finding_count,
          duration_ms, started_at, completed_at, created_at;

-- name: GetSPScan :one
SELECT id, org_id, asset_id, scanner, status,
       target_url, target_ip, error_message, finding_count,
       duration_ms, started_at, completed_at, created_at
FROM vb_scans
WHERE id = $1 AND org_id = $2;

-- name: UpdateSPScanStatus :exec
-- Updates status plus any optional fields. Pass NULL via sqlc.narg() to leave
-- a column unchanged (COALESCE keeps the existing value).
UPDATE vb_scans SET
  status        = $2,
  error_message = COALESCE(sqlc.narg('error_message')::text, error_message),
  finding_count = COALESCE(sqlc.narg('finding_count')::int, finding_count),
  duration_ms   = COALESCE(sqlc.narg('duration_ms')::bigint, duration_ms),
  started_at    = COALESCE(sqlc.narg('started_at')::timestamptz, started_at),
  completed_at  = COALESCE(sqlc.narg('completed_at')::timestamptz, completed_at)
WHERE id = $1;

-- ── Findings (Read + Update) ────────────────────────────────────────────────
-- Upsert/Dedup queries (UpsertFinding, BatchUpsertFindings) remain embedded for
-- now — their business logic (cve_id vs template_id dedup, reopen_count rules,
-- source-set merge) is hard to express as a single sqlc query and currently
-- sits in the repository (Sitzung C2b).

-- name: GetSPFinding :one
SELECT id, org_id, asset_id, scan_id, cve_id,
       title, description, severity,
       cvss_score, epss_score, epss_percentile, risk_score,
       status, scanner, raw_id, sources, template_id,
       assigned_to, justification,
       reopen_count, occurrence_count,
       last_seen_at, sla_due_at, created_at, updated_at
FROM vb_findings
WHERE id = $1 AND org_id = $2;

-- name: CountSPFindings :one
SELECT COUNT(*) FROM vb_findings
WHERE org_id = $1
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('status')::text   IS NULL OR status   = sqlc.narg('status')::text)
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id')::uuid);

-- name: ListSPFindingsByRisk :many
SELECT id, org_id, asset_id, scan_id, cve_id,
       title, description, severity,
       cvss_score, epss_score, epss_percentile, risk_score,
       status, scanner, raw_id, sources, template_id,
       assigned_to, justification,
       reopen_count, occurrence_count,
       last_seen_at, sla_due_at, created_at, updated_at
FROM vb_findings
WHERE org_id = $1
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('status')::text   IS NULL OR status   = sqlc.narg('status')::text)
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id')::uuid)
ORDER BY risk_score DESC NULLS LAST, created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSPFindingsByCreated :many
SELECT id, org_id, asset_id, scan_id, cve_id,
       title, description, severity,
       cvss_score, epss_score, epss_percentile, risk_score,
       status, scanner, raw_id, sources, template_id,
       assigned_to, justification,
       reopen_count, occurrence_count,
       last_seen_at, sla_due_at, created_at, updated_at
FROM vb_findings
WHERE org_id = $1
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('status')::text   IS NULL OR status   = sqlc.narg('status')::text)
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id')::uuid)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateSPFinding :one
-- Partial update via COALESCE: NULL leaves the column unchanged.
UPDATE vb_findings SET
  status        = COALESCE(sqlc.narg('status')::text, status),
  assigned_to   = COALESCE(sqlc.narg('assigned_to')::uuid, assigned_to),
  justification = COALESCE(sqlc.narg('justification')::text, justification),
  severity      = COALESCE(sqlc.narg('severity')::text, severity),
  updated_at    = NOW()
WHERE id = $1 AND org_id = $2
RETURNING id, org_id, asset_id, scan_id, cve_id,
          title, description, severity,
          cvss_score, epss_score, epss_percentile, risk_score,
          status, scanner, raw_id, sources, template_id,
          assigned_to, justification,
          reopen_count, occurrence_count,
          last_seen_at, sla_due_at, created_at, updated_at;

-- name: BulkUpdateSPFindings :execrows
-- Bulk-Update über IN-Liste via UUID-Array.
UPDATE vb_findings SET
  status      = COALESCE(sqlc.narg('status')::text, status),
  assigned_to = COALESCE(sqlc.narg('assigned_to')::uuid, assigned_to),
  updated_at  = NOW()
WHERE org_id = $1 AND id = ANY(sqlc.arg('ids')::uuid[]);

-- ── Suppression Rules ───────────────────────────────────────────────────────

-- name: CreateSPSuppression :one
INSERT INTO vb_finding_suppressions (org_id, cve_id, asset_tag, reason, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, cve_id, asset_tag, reason, created_by, match_count, created_at;

-- name: ListSPSuppressions :many
SELECT id, org_id, cve_id, asset_tag, reason, created_by, match_count, created_at
FROM vb_finding_suppressions
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 10000;

-- name: DeleteSPSuppression :execrows
DELETE FROM vb_finding_suppressions
WHERE id = $1 AND org_id = $2;

-- ── Scan Schedules ──────────────────────────────────────────────────────────

-- name: CreateSPScanSchedule :one
INSERT INTO vb_scan_schedules (org_id, asset_id, scanner, cron_expr, is_active)
VALUES ($1, $2, $3, $4, TRUE)
RETURNING id, org_id, asset_id, scanner, cron_expr, is_active,
          last_run, next_run, created_at;

-- name: ListSPScanSchedules :many
SELECT id, org_id, asset_id, scanner, cron_expr, is_active,
       last_run, next_run, created_at
FROM vb_scan_schedules
WHERE org_id = $1 AND asset_id = $2
ORDER BY created_at DESC
LIMIT 500;

-- name: DeleteSPScanSchedule :execrows
DELETE FROM vb_scan_schedules
WHERE id = $1 AND org_id = $2;

-- ── Risk Trend ──────────────────────────────────────────────────────────────

-- name: GetSPRiskTrend :many
-- Aggregates open findings per day for the last :days days.
SELECT
    TO_CHAR(d::date, 'YYYY-MM-DD')                 AS date,
    COALESCE(SUM(f.risk_score), 0)::float8         AS total_risk_score,
    COUNT(f.id)::int                                AS open_count,
    COUNT(f.id) FILTER (WHERE f.severity = 'critical')::int AS critical_count
FROM generate_series(
    (NOW() - make_interval(days => $2::int))::date,
    NOW()::date,
    '1 day'::interval
) AS d
LEFT JOIN vb_findings f
    ON f.org_id = $1
   AND f.status = 'open'
   AND f.created_at < (d::date + INTERVAL '1 day')
GROUP BY d
ORDER BY d;

-- ── Reports ─────────────────────────────────────────────────────────────────

-- name: CreateSPReport :one
INSERT INTO vb_reports (org_id, generated_by, scope, status)
VALUES ($1, $2, $3, 'pending')
RETURNING id, org_id, generated_by, scope, file_path, status, expires_at, created_at;

-- name: GetSPReport :one
SELECT id, org_id, generated_by, scope, file_path, status, expires_at, created_at
FROM vb_reports
WHERE id = $1 AND org_id = $2;

-- name: ListSPReports :many
SELECT id, org_id, generated_by, scope, file_path, status, expires_at, created_at
FROM vb_reports
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT 100;

-- name: UpdateSPReport :exec
UPDATE vb_reports
SET file_path = $2, status = $3, expires_at = $4
WHERE id = $1;

-- name: StoreSPReportContent :exec
UPDATE vb_reports
SET content = $2, status = 'completed', expires_at = $3, file_path = ''
WHERE id = $1;

-- name: GetSPReportContent :one
SELECT content, scope FROM vb_reports
WHERE id = $1 AND org_id = $2 AND status = 'completed';

-- ── Findings Upsert (single, by raw_id) ─────────────────────────────────────

-- name: UpsertSPFindingByRawID :one
-- Single-shot INSERT...ON CONFLICT for importer flows (SARIF/CycloneDX/CSV).
-- Conflict key: (org_id, raw_id, scanner). Multi-row Upserts with conditional
-- dedup keys (cve_id vs template_id) remain procedural in the repo because
-- the conditional INSERT-vs-UPDATE branch isn't expressible in a single sqlc query.
INSERT INTO vb_findings
  (org_id, asset_id, cve_id, title, description, severity,
   cvss_score, status, scanner, raw_id, sources, sla_due_at,
   reopen_count, occurrence_count, last_seen_at)
VALUES
  ($1, $2, $3, $4, $5, $6,
   $7, $8, $9, $10, $11, $12,
   0, 1, NOW())
ON CONFLICT (org_id, raw_id, scanner) DO UPDATE
  SET title            = EXCLUDED.title,
      description      = EXCLUDED.description,
      severity         = EXCLUDED.severity,
      cvss_score       = EXCLUDED.cvss_score,
      sla_due_at       = EXCLUDED.sla_due_at,
      occurrence_count = vb_findings.occurrence_count + 1,
      last_seen_at     = NOW(),
      updated_at       = NOW()
RETURNING id, org_id, asset_id, scan_id, cve_id,
          title, description, severity,
          cvss_score, epss_score, epss_percentile, risk_score,
          status, scanner, raw_id, sources, template_id,
          assigned_to, justification,
          reopen_count, occurrence_count,
          last_seen_at, sla_due_at, created_at, updated_at;

-- ── SBOM ────────────────────────────────────────────────────────────────────

-- name: CreateSPSBOM :one
INSERT INTO vb_sboms (org_id, asset_id, format, spec_version, document, component_count)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: InsertSPComponent :exec
-- Upsert variant — duplicate (sbom_id, name, version) is a no-op (DO NOTHING).
INSERT INTO vb_components (org_id, sbom_id, name, version, purl)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (sbom_id, name, version) DO NOTHING;

-- name: GetLatestSPSBOM :one
SELECT id, asset_id, format, component_count, created_at
FROM vb_sboms
WHERE org_id = $1 AND asset_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListSPComponentsBySBOM :many
SELECT id, name, version
FROM vb_components
WHERE sbom_id = $1
LIMIT 10000;

-- name: ListSPComponentsAll :many
SELECT c.id, c.name, c.version, c.purl, c.eol_status, c.eol_date, s.asset_id
FROM vb_components c
JOIN vb_sboms s ON s.id = c.sbom_id
WHERE c.org_id = $1
ORDER BY c.name, c.version
LIMIT $2 OFFSET $3;

-- name: ListSPComponentsEOL :many
SELECT c.id, c.name, c.version, c.purl, c.eol_status, c.eol_date, s.asset_id
FROM vb_components c
JOIN vb_sboms s ON s.id = c.sbom_id
WHERE c.org_id = $1 AND c.eol_status = 'eol'
ORDER BY c.name, c.version
LIMIT $2 OFFSET $3;

-- name: UpdateSPComponentEOL :exec
UPDATE vb_components
SET eol_status     = $2,
    eol_date       = sqlc.narg('eol_date')::date,
    eol_checked_at = NOW()
WHERE id = $1;

-- ── EOL Cache ───────────────────────────────────────────────────────────────

-- name: GetSPEOLCache :one
SELECT payload, fetched_at
FROM vb_eol_cache
WHERE product = $1 AND cycle = $2;

-- name: UpsertSPEOLCache :exec
INSERT INTO vb_eol_cache (product, cycle, payload, fetched_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (product, cycle) DO UPDATE
  SET payload    = EXCLUDED.payload,
      fetched_at = EXCLUDED.fetched_at;

-- name: ListSPComponentsBySBOMFull :many
SELECT c.id::text, c.name, c.version, COALESCE(c.purl, '') AS purl,
       c.eol_status,
       CASE WHEN c.eol_date IS NOT NULL THEN c.eol_date::text ELSE NULL END AS eol_date,
       s.asset_id::text AS asset_id
FROM vb_components c
JOIN vb_sboms s ON s.id = c.sbom_id
WHERE c.sbom_id = $1::uuid
ORDER BY c.name, c.version;

-- name: GetSPScanOrgID :one
SELECT org_id::text FROM vb_scans WHERE id = $1::uuid;

-- name: BatchUpdateSPComponentEOL :exec
UPDATE vb_components AS c
SET eol_status     = v.status,
    eol_date       = v.eol_date::date,
    eol_checked_at = NOW()
FROM (
    SELECT
        UNNEST($1::uuid[])  AS id,
        UNNEST($2::text[])  AS status,
        UNNEST($3::text[])  AS eol_date
) AS v
WHERE c.id = v.id;
