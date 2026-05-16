-- migrate: no transaction
DROP INDEX CONCURRENTLY IF EXISTS idx_ck_controls_domain_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_ck_controls_title_trgm;
DROP INDEX CONCURRENTLY IF EXISTS idx_po_avvs_status_review;
DROP INDEX CONCURRENTLY IF EXISTS idx_ck_evidence_expires;
DROP INDEX CONCURRENTLY IF EXISTS idx_vb_findings_sla_overdue;
