-- SecVault queries — migrated to sqlc in Sprint 11+ (ADR-0005, inkrementell).
--
-- Migrationspfad:
--   ✅ Projects, Environments, AccessLog, RotationPolicies — sqlc (this file)
--   ⏳ Secrets — bleibt embedded SQL in repository.go (Crypto-Felder + dynamische
--      Spalten-Auswahl je nach decrypt-Strategie machen sqlc-Generierung holprig).
--      Migration on-demand bei nächstem Secrets-Refactor.

-- ── Projects ────────────────────────────────────────────────────────────────

-- name: CreateSVProject :one
INSERT INTO so_projects (org_id, name, slug, description, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, org_id, name, slug, description, created_by, created_at;

-- name: ListSVProjects :many
SELECT id, org_id, name, slug, description, created_by, created_at
FROM so_projects
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetSVProject :one
SELECT id, org_id, name, slug, description, created_by, created_at
FROM so_projects
WHERE id = $1 AND org_id = $2;

-- name: DeleteSVProject :execrows
DELETE FROM so_projects WHERE id = $1 AND org_id = $2;

-- ── Environments ────────────────────────────────────────────────────────────

-- name: CreateSVEnvironment :one
INSERT INTO so_environments (project_id, org_id, name)
VALUES ($1, $2, $3)
RETURNING id, project_id, org_id, name, created_at;

-- name: ListSVEnvironments :many
SELECT id, project_id, org_id, name, created_at
FROM so_environments
WHERE project_id = $1 AND org_id = $2
ORDER BY name;

-- name: GetSVEnvironment :one
SELECT id, project_id, org_id, name, created_at
FROM so_environments
WHERE id = $1 AND org_id = $2;

-- name: DeleteSVEnvironment :execrows
DELETE FROM so_environments WHERE id = $1 AND org_id = $2;

-- ── Access Log ──────────────────────────────────────────────────────────────

-- name: InsertSVAccessLog :exec
INSERT INTO so_access_log
  (secret_id, org_id, accessed_by, access_via, ip_address, user_agent)
VALUES
  ($1, $2, $3, $4, $5, $6);

-- name: ListSVAccessLog :many
SELECT id, secret_id, org_id, accessed_by, access_via,
       ip_address, user_agent, accessed_at
FROM so_access_log
WHERE org_id = $1
ORDER BY accessed_at DESC
LIMIT 500;

-- name: GetSVAccessLog :many
SELECT id::text, secret_id::text,
       accessed_by::text, access_via,
       ip_address, user_agent, accessed_at
FROM so_access_log
WHERE secret_id = $1::uuid
  AND org_id = $2::uuid
ORDER BY accessed_at DESC
LIMIT $3 OFFSET $4;

-- name: CountSVProjectAccessLog :one
SELECT COUNT(*)::int
FROM so_access_log al
JOIN so_secrets s ON s.id = al.secret_id
JOIN so_environments e ON e.id = s.environment_id
WHERE e.project_id = $1::uuid
  AND al.org_id = $2::uuid;

-- name: ListSVProjectAccessLog :many
SELECT
    al.id::text,
    s.key AS secret_key,
    al.access_via,
    al.accessed_by::text,
    al.ip_address,
    al.accessed_at
FROM so_access_log al
JOIN so_secrets s ON s.id = al.secret_id
JOIN so_environments e ON e.id = s.environment_id
WHERE e.project_id = $1::uuid
  AND al.org_id = $2::uuid
ORDER BY al.accessed_at DESC
LIMIT $3 OFFSET $4;

-- ── Secrets (metadata only — no encrypted_value) ────────────────────────────

-- name: UpdateSVSecretAccess :exec
UPDATE so_secrets
SET access_count     = access_count + 1,
    last_accessed_at = NOW()
WHERE id = $1::uuid;

-- name: ListSVSecretKeys :many
SELECT id::text, key, version, rotation_due_at, last_rotated_at, last_accessed_at,
       access_count, created_at, updated_at
FROM so_secrets
WHERE environment_id = $1::uuid AND org_id = $2::uuid
ORDER BY key ASC;

-- name: DeleteSVSecret :execrows
DELETE FROM so_secrets
WHERE environment_id = $1::uuid AND org_id = $2::uuid AND key = $3;

-- name: GetSVSecretIDByKey :one
SELECT id::text FROM so_secrets
WHERE environment_id = $1::uuid AND org_id = $2::uuid AND key = $3;

-- name: GetSVSecretByKey :one
SELECT id::text, key, version,
       rotation_due_at, last_rotated_at, last_accessed_at,
       access_count, created_at, updated_at
FROM so_secrets
WHERE environment_id = $1::uuid AND key = $2 AND org_id = $3::uuid;

-- name: ListSVProjectSecrets :many
SELECT s.id::text, s.key, s.version,
       s.rotation_due_at, s.last_rotated_at, s.last_accessed_at,
       s.access_count, s.created_at, s.updated_at
FROM so_secrets s
JOIN so_environments e ON e.id = s.environment_id
WHERE e.project_id = $1::uuid AND s.org_id = $2::uuid
ORDER BY s.key ASC;

-- ── Share links ─────────────────────────────────────────────────────────────

-- name: CreateSVShareLink :one
INSERT INTO so_share_links (secret_id, org_id, token_hash, expires_at, created_by)
VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)
RETURNING id::text, secret_id::text, expires_at, created_at;

-- name: GetSVShareLink :one
SELECT id::text, secret_id::text, expires_at, used_at, created_at
FROM so_share_links
WHERE token_hash = $1;

-- name: GetSVShareLinkOrgID :one
SELECT org_id::text FROM so_share_links WHERE id = $1::uuid;

-- name: MarkSVShareLinkUsed :exec
UPDATE so_share_links SET used_at = NOW()
WHERE id = $1::uuid AND org_id = $2::uuid;

-- ── API tokens (api_keys table) ─────────────────────────────────────────────

-- name: CreateSVAPIToken :one
INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes, expires_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
RETURNING id::text, name, key_prefix, scopes, expires_at, last_used_at, revoked_at, created_at;

-- name: ListSVAPITokens :many
SELECT id::text, name, key_prefix, scopes, expires_at, last_used_at, revoked_at, created_at
FROM api_keys
WHERE org_id = $1::uuid AND created_by = $2::uuid
  AND $3 = ANY(scopes)
ORDER BY created_at DESC;

-- name: RevokeSVAPIToken :execrows
UPDATE api_keys
SET revoked_at = NOW()
WHERE id = $1::uuid AND org_id = $2::uuid AND created_by = $3::uuid
  AND $4 = ANY(scopes)
  AND revoked_at IS NULL;

-- ── Environment helpers ──────────────────────────────────────────────────────

-- name: GetSVEnvProjectID :one
SELECT project_id::text FROM so_environments WHERE id = $1::uuid AND org_id = $2::uuid;

-- name: GetSVSecretProjectID :one
SELECT e.project_id::text
FROM so_secrets s
JOIN so_environments e ON e.id = s.environment_id
WHERE s.id = $1::uuid;

-- ── Git scans ────────────────────────────────────────────────────────────────

-- name: CreateSVGitScan :one
INSERT INTO so_git_scans (org_id, repo_url, branch)
VALUES ($1::uuid, $2, $3)
RETURNING id::text, org_id::text, repo_url, branch, status,
          finding_count, open_count, dismissed_count,
          COALESCE(error_message,'') AS error_message, scanned_at, created_at;

-- name: GetSVGitScan :one
SELECT id::text, org_id::text, repo_url, branch, status,
       finding_count, open_count, dismissed_count,
       COALESCE(error_message,'') AS error_message, scanned_at, created_at
FROM so_git_scans WHERE id=$1::uuid AND org_id=$2::uuid;

-- name: ListSVGitScans :many
SELECT id::text, org_id::text, repo_url, branch, status,
       finding_count, open_count, dismissed_count,
       COALESCE(error_message,'') AS error_message, scanned_at, created_at
FROM so_git_scans WHERE org_id=$1::uuid ORDER BY created_at DESC;

-- name: UpdateSVGitScanStatus :exec
UPDATE so_git_scans
SET status=$1, finding_count=$2, open_count=$3, dismissed_count=$4,
    error_message=$5, scanned_at=$6
WHERE id=$7::uuid AND org_id=$8::uuid;

-- ── Git scan results ─────────────────────────────────────────────────────────

-- name: GetSVScanResults :many
SELECT id::text, org_id::text, scan_id::text, repo_url,
       COALESCE(commit_hash,'') AS commit_hash,
       file_path, COALESCE(line_number,0) AS line_number,
       pattern_name, match_preview, severity, status,
       COALESCE(dismiss_reason,'') AS dismiss_reason,
       dismiss_count, created_at
FROM so_scan_results
WHERE scan_id=$1::uuid AND org_id=$2::uuid
ORDER BY severity, file_path;

-- name: DismissSVScanResult :execrows
UPDATE so_scan_results
SET status='dismissed', dismiss_reason=$1, dismiss_count=dismiss_count+1
WHERE id=$2::uuid AND org_id=$3::uuid;

-- name: CountSVDismissals :one
SELECT COALESCE(SUM(dismiss_count), 0)::int
FROM so_scan_results
WHERE org_id=$1::uuid AND pattern_name=$2 AND file_path=$3 AND status='dismissed';

-- ── Rotation policies ────────────────────────────────────────────────────────

-- name: UpsertSVRotationPolicy :one
INSERT INTO so_rotation_policies (org_id, secret_id, interval_days, next_rotation_at)
VALUES ($1::uuid, $2::uuid, $3, $4)
ON CONFLICT (secret_id) DO UPDATE
  SET interval_days=EXCLUDED.interval_days,
      next_rotation_at=EXCLUDED.next_rotation_at
RETURNING id::text, org_id::text, secret_id::text, interval_days,
          last_rotated_at, next_rotation_at, is_active, created_at;

-- name: GetSVRotationPolicy :one
SELECT id::text, org_id::text, secret_id::text, interval_days,
       last_rotated_at, next_rotation_at, is_active, created_at
FROM so_rotation_policies WHERE secret_id=$1::uuid AND org_id=$2::uuid;

-- name: UpdateSVRotationAfterRotate :exec
UPDATE so_rotation_policies
SET last_rotated_at=NOW(), next_rotation_at=$1
WHERE secret_id=$2::uuid AND org_id=$3::uuid;
