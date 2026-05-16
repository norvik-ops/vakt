-- 060: AVV template fields and SCC support
ALTER TABLE po_avvs
    ADD COLUMN IF NOT EXISTS template_id      TEXT,
    ADD COLUMN IF NOT EXISTS body             TEXT,
    ADD COLUMN IF NOT EXISTS scc_module       TEXT
        CHECK (scc_module IN ('module_1','module_2','module_3','module_4')),
    ADD COLUMN IF NOT EXISTS scc_annex_i      TEXT,
    ADD COLUMN IF NOT EXISTS scc_annex_ii     TEXT,
    ADD COLUMN IF NOT EXISTS scc_annex_iii    TEXT;
