# DB-User-Migration: sechealth → vakt

Bestehende Self-Hosted-Deployments, die noch den alten PostgreSQL-User `sechealth` verwenden, müssen diesen einmalig auf `vakt` umbenennen. Migration 141 erledigt das automatisch beim nächsten Start — diese Anleitung erklärt den manuellen Weg sowie was bei automatischen Upgrades passiert.

## Betrifft dich das?

Prüfen:

```bash
docker compose exec postgres psql -U postgres -c "\du" | grep sechealth
```

Wenn die Ausgabe leer ist (oder `sechealth` nicht erscheint), ist der User bereits umbenannt — nichts zu tun.

## Automatischer Weg (empfohlen)

Migration 141 enthält ein idempotentes `DO $$`-Skript, das beim nächsten `docker compose up` automatisch ausgeführt wird, sofern `AUTO_MIGRATE=true` gesetzt ist oder der `migrate`-Service läuft:

1. Backup erstellen (sicherheitshalber):
   ```bash
   docker compose exec postgres pg_dump -U sechealth vakt > backup-vor-migration.sql
   ```

2. Neue Version starten:
   ```bash
   docker compose pull
   docker compose up -d
   ```

   Der `migrate`-Container führt Migration 141 aus. Dabei wird geprüft, ob der Role `sechealth` existiert — falls ja, wird er zu `vakt` umbenannt.

3. `VAKT_DB_URL` in `.env` anpassen (falls noch `sechealth` im Username):
   ```
   VAKT_DB_URL=postgres://vakt:DEIN_PASSWORT@postgres:5432/vakt?sslmode=disable
   ```

4. Container neu starten:
   ```bash
   docker compose up -d
   ```

## Manueller Weg

Falls `AUTO_MIGRATE=false` gesetzt ist oder die Migration manuell ausgeführt werden soll:

### Schritt 1 — Vakt stoppen

```bash
docker compose stop api worker
```

### Schritt 2 — PostgreSQL-User umbenennen

```bash
docker compose exec postgres psql -U postgres -c "ALTER USER sechealth RENAME TO vakt;"
```

Hinweis: Dafür wird der PostgreSQL-Superuser (`postgres`) benötigt, nicht der Applikationsuser. Der `postgres`-Superuser ist im `postgres:16-alpine`-Image immer vorhanden.

### Schritt 3 — `.env` anpassen

```bash
# Vorher:
VAKT_DB_URL=postgres://sechealth:PASSWORT@postgres:5432/vakt?sslmode=disable

# Nachher:
VAKT_DB_URL=postgres://vakt:PASSWORT@postgres:5432/vakt?sslmode=disable
```

Das Passwort bleibt unverändert — nur der Username ändert sich.

### Schritt 4 — Vakt starten

```bash
docker compose up -d
```

## Rollback

Falls etwas schiefläuft: Migration 141 hat ein `.down.sql`, das den User zurück zu `sechealth` benennt. Manuell:

```bash
docker compose exec postgres psql -U postgres -c "ALTER USER vakt RENAME TO sechealth;"
```

Danach `VAKT_DB_URL` zurücksetzen und Container neu starten.

## Helm / Kubernetes

Wenn Vakt per Helm deployed wird:

1. `values.yaml` prüfen: `postgresql.auth.username` sollte `vakt` sein.
2. Bestehenden Postgres-User umbenennen (wie oben, via `kubectl exec`).
3. Helm-Release updaten: `helm upgrade vakt ./helm/vakt`.

## Hintergrund

Der User `sechealth` stammt aus der Zeit vor dem Produkt-Rebrand (SecHealth → Vakt). `docker-compose.yml` und `docker-compose.dev.yml` verwenden seit v0.6.x bereits `vakt`. `docker-compose.demo.yml` wurde mit Sprint 45 nachgezogen (S45-2). Migration 141 schließt die letzte Lücke für bestehende Deployments, die noch den alten User haben.
