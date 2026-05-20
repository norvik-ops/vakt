-- 122: Vakt Aware Tabellen-Präfix pg_ → sr_ umbenennen.
--
-- Hintergrund: Der `pg_*`-Präfix kollidiert mit dem PostgreSQL-System-
-- Katalog-Namespace, was den sqlc-Parser dazu zwingt, Spaltenreferenzen
-- in eigenen Tabellen als ambiguous abzulehnen (siehe ADR-0005). Damit
-- blieb Vakt Aware als einziges Modul auf embedded SQL. Diese Migration
-- räumt das auf — pg_* → sr_* (Vakt Aware-Modul-Präfix).
--
-- Migration ist eine reine Metadaten-Operation in Postgres (ALTER TABLE
-- RENAME TO ist instant, touch keine Daten, behält Foreign Keys, Indexe,
-- Sequenzen, Trigger). Down-Migration ist trivial (Rename zurück).
--
-- Wichtig für Customers: Diese Migration muss in derselben Release wie
-- der aktualisierte Server-Binary ausgerollt werden — alte Binaries mit
-- hardcodierten `pg_*`-SQL-Strings würden gegen die neuen Tabellennamen
-- failen. Die Vakt-Standard-Auslieferung (docker compose mit migrate +
-- api in einem Image) gewährleistet das automatisch.

ALTER TABLE pg_templates       RENAME TO sr_templates;
ALTER TABLE pg_target_groups   RENAME TO sr_target_groups;
ALTER TABLE pg_targets         RENAME TO sr_targets;
ALTER TABLE pg_landing_pages   RENAME TO sr_landing_pages;
ALTER TABLE pg_campaigns       RENAME TO sr_campaigns;
ALTER TABLE pg_events          RENAME TO sr_events;
ALTER TABLE pg_training_modules RENAME TO sr_training_modules;
ALTER TABLE pg_assignments     RENAME TO sr_assignments;
ALTER TABLE pg_completions     RENAME TO sr_completions;
ALTER TABLE pg_phish_reports   RENAME TO sr_phish_reports;

-- Indexe folgen den Tabellen automatisch, aber die Index-NAMEN behalten
-- den alten Präfix. Konsistenz-halber auch die umbenennen:
ALTER INDEX idx_pg_assignments_target_id RENAME TO idx_sr_assignments_target_id;
ALTER INDEX idx_pg_assignments_module_id RENAME TO idx_sr_assignments_module_id;
ALTER INDEX idx_pg_events_campaign_id    RENAME TO idx_sr_events_campaign_id;
ALTER INDEX idx_pg_events_target_id      RENAME TO idx_sr_events_target_id;
ALTER INDEX idx_pg_targets_group_id      RENAME TO idx_sr_targets_group_id;
ALTER INDEX idx_pg_campaigns_org_id      RENAME TO idx_sr_campaigns_org_id;
ALTER INDEX idx_pg_events_token          RENAME TO idx_sr_events_token;
ALTER INDEX IF EXISTS pg_phish_reports_org_idx RENAME TO sr_phish_reports_org_idx;
