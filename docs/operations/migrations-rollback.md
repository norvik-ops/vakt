# Runbook: Datenbank-Migration Rollback

Dieses Runbook beschreibt, wie eine fehlerhafte PostgreSQL-Migration rückgängig gemacht wird.
Vakt verwendet `golang-migrate` mit SQL-Migrations-Dateien unter `backend/db/migrations/`.

> **Vorsicht:** Migrations mit `DROP COLUMN`, `DROP TABLE` oder `DROP INDEX` sind nach Ausführung
> **nicht reversibel** — die Daten sind weg. Immer zuerst sichern.

---

## Wann ist ein Rollback nötig?

- Eine Migration hat strukturell erfolgreich durchlaufen, verursacht aber App-Fehler (z.B. falsche Column-Typen, fehlendes Index verursacht N+1-Explosionen)
- Eine `.up.sql` enthielt einen Logik-Fehler (falscher Default-Wert, zu restriktives NOT NULL-Constraint)
- Smoke-Tests nach Migration schlagen fehl (`TestOpenAPIContract`, `TestWorkerRawSQLAgainstSchema`)
- Die App befindet sich in einem Restart-Loop nach Deployment

---

## Vorbedingung: Backup erstellen

**IMMER zuerst sichern — vor jeder Rollback-Aktion:**

```bash
docker compose exec postgres pg_dump -U vakt -d vakt \
  > backup-before-rollback-$(date +%Y%m%d%H%M).sql
```

Backup verifizieren (Zeilenzahl sollte > 0 sein):

```bash
wc -l backup-before-rollback-*.sql
```

Für Produktionsinstanzen: Backup auf externen Storage kopieren (S3, off-site), bevor fortgefahren wird.

---

## Aktuellen Migrations-Stand feststellen

```bash
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  version
```

Gibt die zuletzt angewendete Migrations-Versionsnummer aus (z.B. `228`).

Alternativ direkt in der DB:

```bash
docker compose exec postgres psql -U vakt -d vakt \
  -c "SELECT version, dirty FROM schema_migrations;"
```

Wenn `dirty = true`: Migration ist fehlgeschlagen und hängt. Zuerst dirty-Flag bereinigen (siehe unten).

---

## Rollback via golang-migrate

**N Schritte zurück (normalerweise 1):**

```bash
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  down 1
```

`golang-migrate` führt die entsprechende `.down.sql`-Datei aus und decrementiert den Versions-Counter in `schema_migrations`.

**Mehrere Schritte zurück:**

```bash
# 3 Migrationen zurückrollen
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  down 3
```

**Rollback auf eine bestimmte Version (z.B. auf 225):**

```bash
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  goto 225
```

---

## Dirty-Flag bereinigen

Wenn eine Migration halb durchgelaufen ist (z.B. Verbindungsabbruch), bleibt `schema_migrations.dirty = true`. golang-migrate weigert sich in diesem Zustand, weitere Befehle auszuführen.

**Manuell bereinigen (mit Vorsicht):**

```bash
docker compose exec postgres psql -U vakt -d vakt \
  -c "UPDATE schema_migrations SET dirty = false WHERE version = <VERSIONSNUMMER>;"
```

Danach Status prüfen:

```bash
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  version
```

---

## Alternativ: Direktes SQL ausführen

Wenn die `.down.sql` korrekt ist, aber der migrate-Container nicht verfügbar ist:

```bash
docker compose exec postgres psql -U vakt -d vakt
```

Dann in der psql-Session:

```sql
-- Inhalt der .down.sql manuell eingeben
ALTER TABLE ck_controls DROP COLUMN IF EXISTS new_column;

-- schema_migrations manuell decrementieren
UPDATE schema_migrations SET version = 227, dirty = false WHERE version = 228;
```

> **Hinweis:** `schema_migrations` hat immer genau eine Zeile. Die Versionsnummer muss exakt
> der letzten erfolgreich angewendeten Migration entsprechen.

---

## Fehlende `.down.sql` — manuell erstellen

Falls die `.down.sql` für eine Migration fehlt, muss sie manuell geschrieben werden.
Datei-Namensschema: `backend/db/migrations/{VERSION}_{name}.down.sql`

Beispiel — wenn `.up.sql` eine Tabelle erstellt hat:

```sql
-- backend/db/migrations/228_saml_config.down.sql
DROP TABLE IF EXISTS auth_saml_config;
```

Beispiel — wenn `.up.sql` eine Column hinzugefügt hat:

```sql
-- backend/db/migrations/227_oidc_settings.down.sql
ALTER TABLE auth_oidc_config DROP COLUMN IF EXISTS jwks_uri;
```

---

## Backup-Restore als Fallback

Wenn kein funktionsfähiges `.down.sql` existiert und die Daten zu komplex für manuelle Rekonstruktion sind:

1. App stoppen: `docker compose stop api worker`
2. Vollständige DB-Restore: Siehe [`backup-restore.md`](backup-restore.md)
3. App neu starten: `docker compose up -d api worker`

---

## Nach dem Rollback: Migration-Chain verifizieren

Sicherstellen, dass die Migration-Chain nach dem Rollback wieder sauber ist:

```bash
docker compose run --rm migrate \
  -database "$VAKT_DB_URL" \
  -path /migrations \
  up
```

Erwartung: `no change` oder die rolled-back Migration wird sauber erneut angewendet (nur wenn das Problem behoben wurde).

CI-Smoke-Test lokal nachstellen:

```bash
cd backend && go test ./cmd/worker/... -run TestWorkerRawSQLAgainstSchema -v
cd backend && go test ./cmd/api/... -run TestOpenAPIContract -v
```

---

## Was golang-migrate intern trackt

- Tabelle: `schema_migrations` (wird automatisch angelegt bei erster Migration)
- Schema: `version BIGINT, dirty BOOLEAN`
- Rollback decrementiert `version` auf den vorherigen Wert
- Bei `down 1`: version geht von 228 auf 227

---

*Stand: 2026-06-26*
