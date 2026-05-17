-- Performance indexes for frequent filter patterns.

CREATE INDEX IF NOT EXISTS idx_vb_findings_sla_overdue
  ON vb_findings(org_id, sla_due_at)
  WHERE status NOT IN ('resolved', 'false_positive');

CREATE INDEX IF NOT EXISTS idx_ck_evidence_expires
  ON ck_evidence(org_id, expires_at)
  WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_po_avvs_status_review
  ON po_avvs(org_id, status, review_date)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_ck_controls_title_trgm
  ON ck_controls USING GIN (lower(title) gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_ck_controls_domain_trgm
  ON ck_controls USING GIN (lower(domain) gin_trgm_ops);
