-- migrate: no transaction
-- Performance indexes for frequent filter patterns.
-- CONCURRENTLY requires no surrounding transaction block.

-- Für SLA-Overdue-Checks (täglich ausgeführt)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_vb_findings_sla_overdue
  ON vb_findings(org_id, sla_due_at)
  WHERE status NOT IN ('resolved', 'false_positive');

-- Für Evidence-Expiry-Alerts
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ck_evidence_expires
  ON ck_evidence(org_id, expires_at)
  WHERE expires_at IS NOT NULL;

-- Für AVV-Expiry-Checks
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_po_avvs_status_review
  ON po_avvs(org_id, status, review_date)
  WHERE status = 'active';

-- Für Control-Keyword-Suche via GIN/trigram
-- pg_trgm is already enabled by migration 037_search_trgm.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ck_controls_title_trgm
  ON ck_controls USING GIN (lower(title) gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ck_controls_domain_trgm
  ON ck_controls USING GIN (lower(domain) gin_trgm_ops);
