# ADR-0004: Modul-Isolation via Go-Package und DB-Prefix

**Status:** Accepted  
**Datum:** 2026-02-01

## Kontext

Vakt hat 6 Module (Vakt Comply, Vakt Scan, Vakt Vault, Vakt Aware, Vakt Privacy, Vakt HR). Sie sollen:

- Per `VAKT_MODULES_ENABLED` einzeln deaktivierbar sein
- Unabhängig getestet, gewartet und versioniert werden
- Sich nicht gegenseitig kompromittieren (Bug in einem Modul ≠ Datenleck in anderem)

Wir hätten getrennte Microservices nehmen können. Bei self-hosted KMU-Setups ist das aber komplexitätsmäßig Overkill — der Kunde will EIN `docker compose up`.

## Entscheidung

**Modul-Isolation auf zwei Ebenen, alles im selben Monolith-Binary:**

1. **Go-Package-Boundary**: jedes Modul lebt unter `backend/internal/modules/<name>/`. Cross-Modul-Imports sind verboten (CI-Lint).
2. **DB-Tabellen-Prefix**: jedes Modul hat seinen eigenen Tabellen-Prefix:
   - `ck_` Vakt Comply (Package `vaktcomply`, compliance kit)
   - `vb_` / `vaktscan_` Vakt Scan (Package `vaktscan`, vulnerability board)
   - `so_` Vakt Vault (Package `vaktvault`, secret ops)
   - `pg_` Vakt Aware (Package `vaktaware`, phishguard — siehe ADR-0005 für Folgeproblem)
   - `po_` Vakt Privacy (Package `vaktprivacy`, privacy ops)
   - `hr_` Vakt HR (Package `hr`)

Cross-Modul-Kommunikation läuft ausschließlich über **Interfaces in `shared/`** (siehe `EvidenceWriter` für Vakt HR → Vakt Comply).

## Alternativen

- **Microservices** — verworfen: Operativer Overhead für KMU-Kunden zu hoch.
- **Plugin-Mechanismus mit Go-Plugins** — verworfen: `plugin`-Paket ist auf Linux limitiert, ABI-fragil, schlechte DX.
- **Ein gemeinsames DB-Prefix** — verworfen: macht Modul-Abschaltung schwer; macht Schema-Browser unleserlich.

## Konsequenzen

### Positive

- Klare Modul-Grenzen im Code-Tree und in der DB.
- `VAKT_MODULES_ENABLED` schaltet Routes UND Worker-Jobs aus.
- Tests jeden Moduls in ~3 Sekunden ohne andere Module zu involvieren.

### Negative

- 35+ Subdirectories in `internal/shared/` — Tendenz zur „God-Library". Siehe ADR-0011 (sollte aufgeteilt werden, wenn die Schmerzen zu groß werden).
- DB-Prefixes mit `pg_` kollidieren mit PostgreSQL-System-Katalog-Namespace — sqlc kann diese Tabellen nicht parsen (siehe ADR-0005).

### Neutrale

- Kein gemeinsamer ORM-Layer — jedes Modul nutzt sqlc oder embedded SQL.

## Referenzen

- `backend/internal/modules/*/`
- ADR-0005 (sqlc + pg_-Problem)
