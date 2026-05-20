-- 120 down: nur die Indexe wieder entfernen.
-- Die Dedup-DELETEs aus der up-Migration sind irreversibel — Daten, die als
-- Duplikate gelöscht wurden, sind weg. Das ist akzeptabel, weil Duplikate
-- auf diesen Schlüsseln semantisch identisch behandelt werden sollen
-- (UpsertFindingByRawID / BatchUpsertFindings dedupen sie ohnehin).
DROP INDEX IF EXISTS idx_vb_findings_dedup_rawid;
DROP INDEX IF EXISTS idx_vb_findings_dedup_template;
DROP INDEX IF EXISTS idx_vb_findings_dedup_cve;
