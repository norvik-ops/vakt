# ADR-0005: sqlc modulweise einführen, keine Vollmigration

**Status:** Accepted  
**Datum:** 2026-05-19

## Kontext

Die Projekt-Konvention lautet: „Use sqlc for all database queries — no raw string concatenation." Die Realität war bis Mai 2026: 1 sqlc-Query-File, ~609 Zeilen embedded SQL über das Codebase verteilt. Eine externe Code-Analyse markierte das als P0-Tech-Debt.

Eine Vollmigration aller ~30 Repositories in einem Pass ist 6–10 Sprints und birgt erhebliches Regressionsrisiko (touche jede Datenbank-Operation). Zugleich blockiert die `pg_`-Tabellen-Konvention (Vakt Aware, Package `secreflex`) den sqlc-Parser: PostgreSQL behandelt `pg_*` als reservierten System-Katalog-Namespace, sqlc kann hier Spalten-Referenzen nicht zuverlässig disambiguieren („column reference 'org_id' is ambiguous" bei Single-Table-INSERT).

## Entscheidung

**Schrittweise Migration modul-für-modul** statt Big-Bang:

1. Neue Queries werden ausschließlich als sqlc-Datei in `db/queries/` geschrieben.
2. Existing Module werden migriert, wenn das Modul ohnehin angefasst wird oder ein konkreter Need entsteht (z.B. Refactor, neuer Endpoint).
3. **Vakt HR** als erstes migriert (Mai 2026) — Validierung des Patterns.
4. **Vakt Aware** wurde initial wegen `pg_`-Prefix-Bug ausgeklammert. **Update 2026-05-20:** Migration 122 hat alle `pg_*`-Tabellen auf `sr_*` umbenannt (reine Metadaten-Operation in Postgres, behält FKs/Indexe/Sequenzen). Damit konnte sqlc die Tabellen parsen und Vakt Aware wurde ebenfalls migriert. Siehe `docs/sqlc-migration-plan.md`.
5. **Vakt Vault** als nächster Kandidat (Prefix `so_`, keine Kollision).

## Alternativen

- **Big-Bang-Vollmigration in 6 Sprints** — verworfen: blockiert alle anderen Sprints, hohes Regressionsrisiko, kein Kundennutzen heute.
- **Pflicht-Tabellen-Rename `pg_` → `sr_`** — erwogen, aber Migration auf jeder Kunden-Instanz mit Rollback-Risiko. Aufgeschoben bis sqlc-Upstream entscheidet.
- **Anderes Tool als sqlc** (z.B. squirrel als Query-Builder, oder gen) — verworfen: sqlc ist als Projekt-Standard verankert, Tool-Wechsel würde bestehendes Wissen vernichten.

## Konsequenzen

### Positive

- Migration ohne Stillstand der Feature-Entwicklung.
- Jedes neue Modul/Feature trägt zur Tilgung bei.
- Pattern in `backend/internal/modules/hr/repository.go` als Vorlage etabliert.

### Negative

- Inkonsistenz während der Übergangszeit: einige Module nutzen sqlc, andere embedded SQL — neue Entwickler müssen beide Pattern verstehen. **Seit 2026-05-20: vollständig aufgelöst, alle Module migriert.**

### Neutrale

- `sqlc.yaml` hat `emit_exact_table_names: true` (sonst kollidieren `audit_log` und `audit_logs` zu `AuditLog`).

## Referenzen

- `backend/internal/modules/hr/repository.go` (Referenz-Implementierung)
- `backend/internal/modules/secreflex/repository.go` (sqlc-Migration seit Mai 2026)
- `backend/db/migrations/122_rename_pg_tables_to_sr.up.sql` (Tabellen-Rename)
- `backend/sqlc.yaml`
- `docs/sqlc-migration-plan.md` (vollständiger Migrationsstatus)
- sqlc-Migration vollständig abgeschlossen: alle Module migriert (Stand: Mai 2026)
