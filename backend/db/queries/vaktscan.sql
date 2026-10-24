-- Asset inventory queries (vb_assets, owned by Vakt Comply since
-- docs/stories/module-archivieren.md S1). The remaining Vakt Scan queries were
-- removed with the archived module (tag archiv/module-vollstaendig).

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
       created_at, updated_at,
       COALESCE(classification, '')::text AS classification
FROM vb_assets
WHERE org_id = $1 AND is_deleted = FALSE
  AND (sqlc.narg('tag')::text IS NULL OR sqlc.narg('tag')::text = ANY(tags))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetSPAsset :one
SELECT id, org_id, name, type, criticality, tags, owner_id, external_url,
       created_at, updated_at,
       COALESCE(classification, '')::text AS classification
FROM vb_assets
WHERE id = $1 AND org_id = $2 AND is_deleted = FALSE;

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
