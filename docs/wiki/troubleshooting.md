# Troubleshooting

Häufige Probleme und ihre Lösung. Für operative Runbooks (Backup-Restore, Migrations-Rollback, Redis-Ausfall) siehe [`docs/operations/`](../operations/README.md).

---

## IP-Lockout: Anmeldung für alle gesperrt

**Symptom:** Alle Nutzer aus einem Netzwerk erhalten `429 {"code":"IP_LOCKED"}` beim Login.

**Ursache:** Ein Nutzer hat zu viele Anmeldeversuche mit falschen Credentials gemacht. Ab 50 Fehlversuchen von einer IP (konfigurierbar via `VAKT_RATELIMIT_IP_MAX`) wird die gesamte IP für 15 Minuten gesperrt.

**Diagnose:**
```bash
# Welche IPs sind gesperrt?
redis-cli KEYS 'login_fail_ip:*'

# Konkreter Zähler einer IP
redis-cli GET 'login_fail_ip:203.0.113.5'

# (IP, Email)-Paar-Sperren (primäre Sperre — NAT-sicher)
redis-cli KEYS 'login_fail_ip_email:*'
```

**Sofort-Lösung (manuelle Entsperrung):**
```bash
# Einzelne IP entsperren
redis-cli DEL 'login_fail_ip:203.0.113.5'

# Alle IP-Sperren löschen (nur in akuten Notfällen)
redis-cli KEYS 'login_fail_ip:*' | xargs redis-cli DEL
redis-cli KEYS 'login_fail_ip_email:*' | xargs redis-cli DEL
```

**Dauerlösung:** `VAKT_RATELIMIT_IP_MAX` erhöhen für Corporate-NAT-Umgebungen. 15 Minuten abwarten ist die harmloseste Option.

---

## Rate-Limit 429 — Demo-Start oder Login

**Demo-Start (`POST /api/v1/demo/start`):**
- Limit: 10 Starts pro 5-Minuten-Fenster pro IP (Fixed-Window).
- Ursache: Mehr als 10 Klicks auf „Demo starten" in 5 Minuten.
- Lösung: 5 Minuten warten.

**Login (`POST /api/v1/auth/login`):**
- Primär: 10 Fehlversuche pro (IP, Email)-Paar → 15 Minuten gesperrt (nur das Konto, nicht die IP).
- Sekundär: 50 Fehlversuche von einer IP insgesamt → 15 Minuten gesperrt (alle Konten dieser IP).
- Lösung: Warten oder Redis-Key löschen (siehe IP-Lockout oben).

**Load-Tests / CI:** Load-Tests gegen Demo-Endpoints brauchen Sleep zwischen Requests. Produktive Last-Tests auf einer privaten Staging-Instanz mit höheren Limits durchführen. Siehe [`loadtest/README.md`](../../loadtest/README.md).

---

## Demo-Flow kaputt: Login funktioniert nicht

**Symptom:** Klick auf „Demo starten" → Login-Maske zeigt keine Credentials, oder Login schlägt mit `invalid credentials` fehl.

**Diagnose (in der Reihenfolge abarbeiten):**

1. **API-Container läuft?**
   ```bash
   docker compose ps vakt-api
   docker compose logs vakt-api --tail=50
   ```
   Restart-Loop (Status `Restarting`) deutet auf fehlende Env-Vars oder DB-Connection-Fehler.

2. **Demo-Modus aktiviert?**
   ```bash
   grep VAKT_DEMO .env   # muss "true" sein
   ```

3. **Ephemerer Demo-Flow:**
   ```bash
   curl -sX POST http://localhost/api/v1/demo/start | jq .
   ```
   Antwort muss `admin_email`, `admin_password`, `analyst_email`, `analyst_password` enthalten.
   - Kein JSON → API antwortet nicht (Container-Problem, Step 1).
   - `{"error": "..."}` → Migrations nicht durchgelaufen oder DB-Fehler.

4. **Migrations-Status:**
   ```bash
   docker compose logs migrate --tail=20
   # oder:
   docker compose run --rm -e AUTO_MIGRATE=true vakt-api
   ```

**Häufigste Ursachen:**
- `AUTO_MIGRATE=false` (Default): Migrations nicht gelaufen → DB-Schema fehlt.
- `VAKT_SECRET_KEY` fehlt oder ungültig → API startet nicht.
- `VAKT_DB_URL` zeigt auf falsche DB.

---

## Migrations-Fehler

**Symptom:** `docker compose logs migrate` zeigt Fehler, oder die API startet mit DB-Fehlern.

**Fehlercodes:**

| Code | Ursache | Lösung |
|------|---------|--------|
| `SQLSTATE 42P17` | `NOW()`/volatile Funktion in Partial-Index-WHERE | Index ohne volatile Funktion neu schreiben |
| `SQLSTATE 25001` | `CREATE INDEX CONCURRENTLY` in Transaktion | `CONCURRENTLY` entfernen oder `-- migrate: no transaction` setzen |
| `dirty: true` in golang-migrate | Vorherige Migration mit Fehler abgebrochen | Dirty-Flag manuell zurücksetzen (s.u.) |
| `no change` | Migration bereits angewendet | Kein Handlungsbedarf |

**Dirty-Flag zurücksetzen:**
```bash
# ACHTUNG: Nur wenn sicher gestellt ist, dass die fehlgeschlagene Migration vollständig zurückgerollt wurde
docker compose run --rm -e VAKT_DB_URL=$VAKT_DB_URL vakt-api \
    golang-migrate -database "$VAKT_DB_URL" -path /app/migrations force VERSION
```

Vollständiges Rollback-Verfahren: [`docs/operations/migrations-rollback.md`](../operations/migrations-rollback.md).

---

## Backup & Restore (Kurzreferenz)

| Szenario | Runbook |
|----------|---------|
| Manuelle Backup-Erstellung | [`backup-restore.md`](../operations/backup-restore.md) |
| Redis-Ausfall | [`redis-failure.md`](../operations/redis-failure.md) |
| Versehentliche Massen-Löschung | [`bulk-deletion-recovery.md`](../operations/bulk-deletion-recovery.md) |
| Vollverlust (Datacenter) | [`restore-from-offsite.md`](../operations/restore-from-offsite.md) |
| Migrations-Rollback | [`migrations-rollback.md`](../operations/migrations-rollback.md) |

---

## Container startet nicht

**Log-Analyse:**
```bash
docker compose logs vakt-api --tail=100
docker compose logs vakt-worker --tail=100
```

**Häufige Ursachen:**

| Symptom im Log | Ursache | Lösung |
|---------------|---------|--------|
| `VAKT_SECRET_KEY: invalid value` | Key fehlt, ist kein 32-Byte-Hex, oder hat wiederholte Bytes | Key neu generieren: `openssl rand -hex 32` |
| `failed to connect to database` | DB noch nicht bereit, oder `VAKT_DB_URL` falsch | `docker compose logs vakt-db` prüfen; DB zuerst starten |
| `permission denied: /data/uploads` | Upload-Volume falsche Berechtigung | `docker volume rm vakt_uploads && docker compose up -d` |
| `address already in use :8080` | Port belegt von anderem Prozess | `lsof -i :8080` → Prozess beenden oder `VAKT_API_PORT` ändern |
| `bind: cannot assign requested address` | `VAKT_API_PORT` / Netzwerk-Konflikt | Port-Konfiguration prüfen |
| `migrate: no migration files found` | Migrations-Pfad fehlt im Image | Image neu bauen: `docker compose build vakt-api` |

**Health-Check:**
```bash
curl -s http://localhost/health | jq .
# Erwartet: {"status":"ok","version":"...","demo":false,"sso_enabled":false}
```

**Container crasht sofort (Restart-Loop):**
```bash
docker inspect $(docker compose ps -q vakt-api) | jq '.[0].State'
# ExitCode != 0 → Fehler beim Start
docker compose logs vakt-api 2>&1 | grep -E "FATAL|ERROR|panic"
```

---

## Mehr Hilfe

- **Community:** [GitHub Discussions](https://github.com/norvik-ops/vakt/discussions)
- **Bugs:** [GitHub Issues](https://github.com/norvik-ops/vakt/issues)
- **Operative Runbooks:** [`docs/operations/README.md`](../operations/README.md)
- **Konfigurationsreferenz:** [`docs/wiki/configuration.md`](configuration.md)
