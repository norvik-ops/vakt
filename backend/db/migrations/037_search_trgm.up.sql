-- migrate: no transaction
-- Enable pg_trgm for fast infix/fuzzy search via GIN indexes.
-- Replaces the implicit full-table ILIKE scan on each search query.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vb_assets_name_trgm
    ON vb_assets USING GIN (lower(name) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vb_findings_title_trgm
    ON vb_findings USING GIN (lower(title) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ck_risks_title_trgm
    ON ck_risks USING GIN (lower(title) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_po_dsr_requester_trgm
    ON po_dsr USING GIN (lower(requester_name) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_po_breaches_title_trgm
    ON po_breaches USING GIN (lower(title) gin_trgm_ops);
