DROP INDEX IF EXISTS idx_vb_assets_name_trgm;
DROP INDEX IF EXISTS idx_vb_findings_title_trgm;
DROP INDEX IF EXISTS idx_ck_risks_title_trgm;
DROP INDEX IF EXISTS idx_po_dsr_requester_trgm;
DROP INDEX IF EXISTS idx_po_breaches_title_trgm;
DROP EXTENSION IF EXISTS pg_trgm;
