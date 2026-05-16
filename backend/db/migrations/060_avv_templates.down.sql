-- 060 down: remove AVV template and SCC fields
ALTER TABLE po_avvs
    DROP COLUMN IF EXISTS template_id,
    DROP COLUMN IF EXISTS body,
    DROP COLUMN IF EXISTS scc_module,
    DROP COLUMN IF EXISTS scc_annex_i,
    DROP COLUMN IF EXISTS scc_annex_ii,
    DROP COLUMN IF EXISTS scc_annex_iii;
