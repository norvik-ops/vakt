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
