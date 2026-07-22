# Runbook: Redis-Ausfall

Dieses Runbook beschreibt die Diagnose und Behebung eines Redis-Ausfalls in einer Vakt-Instanz.
Redis ist kritisch für Asynq-Job-Queues, Session-Middleware und Rate-Limiter.

---

## Symptome

- Asynq-Jobs werden nicht mehr verarbeitet (keine neuen Tasks in der Queue)
- Login schlägt mit HTTP 500 fehl (Session-Middleware kann Token nicht prüfen)
- Worker-Health-Endpoint `:9090/health` antwortet nicht mehr mit `{"status":"ok"}`
- API-Logs zeigen `redis: connection refused` oder `dial tcp <host>:6379: connect: connection refused`
- Rate-Limiter-Antworten schlagen fehl (könnten als 500 statt 429 erscheinen)

---

## Diagnose

**1. Container-Status prüfen:**

```bash
docker compose ps redis
```

Erwartetes Ergebnis: `running (healthy)`. Bei `Exit` oder `restarting` → direkt zu Neustart-Prozedur.

**2. Letzte Logs:**

```bash
docker compose logs redis --tail=50
```

Häufige Fehler:
- `MISCONF Redis is configured to save RDB snapshots, but it's currently not able to persist on disk` → Disk-Problem
- `Fatal error, can't open config file` → Konfigurations-Fehler beim Start
- `Authentication required` oder `WRONGPASS` → Passwort-Mismatch (siehe unten)

**3. Konnektivität prüfen:**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" ping
```

Erwartete Antwort: `PONG`. Bei `NOAUTH Authentication required` ist `REDIS_PASSWORD` in `.env` nicht gesetzt oder stimmt nicht überein.

**4. Replikation / Info:**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" info replication
```

Bei Standalone-Setup (Standard): `role:master` erwartet. Falls `role:slave` ohne konfigurierten Master → Konfigurationsproblem.

**5. Memory-Auslastung:**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" info memory | grep used_memory_human
```

---

## Neustart-Prozedur

**Schritt 1 — Redis neu starten:**

```bash
docker compose restart redis
```

**Schritt 2 — Konnektivität verifizieren (innerhalb ~10 Sekunden):**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" ping
# Erwartete Antwort: PONG
```

**Schritt 3 — Worker neu verbinden (Asynq reconnect erzwingen):**

```bash
docker compose restart worker
```

**Schritt 4 — Verify Worker-Health:**

```bash
curl -sf http://localhost:9090/health | jq .
# Erwartetes Ergebnis: {"status":"ok"}
```

**Schritt 5 — Asynq-Queue prüfen (optional):**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" llen "asynq:{default}:pending"
```

---

## Häufige Ursache: REDIS_PASSWORD nicht gesetzt

Wenn der Redis-Container mit `requirepass` konfiguriert ist (Standard in Production), aber `REDIS_PASSWORD` in `.env` fehlt oder leer ist, startet Redis, lehnt aber alle Verbindungen mit `NOAUTH` ab.

**Prüfen:**

```bash
grep REDIS_PASSWORD .env
```

**Sicherstellen, dass der Wert in allen Compose-Services übereinstimmt** (`redis`, `api`, `worker`). Nach Korrektur:

```bash
docker compose up -d redis api worker
```

---

## Daten-Recovery bei Volume-Korruption

### RDB-Snapshot (Standard)

Redis speichert Snapshots in `/data/dump.rdb` im Container (Volume `redis_data`).

**Snapshot-Datei prüfen:**

```bash
docker compose exec redis ls -lh /data/dump.rdb
```

**Volume auf dem Host lokalisieren:**

```bash
docker volume inspect vakt_redis_data
# "Mountpoint": "/var/lib/docker/volumes/vakt_redis_data/_data"
```

**Manuelles Backup des Snapshots:**

```bash
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" BGSAVE
cp /var/lib/docker/volumes/vakt_redis_data/_data/dump.rdb \
   /root/backups/redis-dump-$(date +%Y%m%d%H%M%S).rdb
```

### AOF

AOF ist in der Standard-Konfiguration **nicht aktiviert**. Wenn aktiviert, liegt die Datei unter `/data/appendonly.aof`.

### Volume neu erstellen (letzte Option)

Wenn das Volume irreparabel korrupt ist, kann es neu erstellt werden. **Konsequenzen:**

| Was geht verloren | Auswirkung |
|-------------------|------------|
| Aktive Sessions (Paseto-Token-Revocation-Entries) | Alle eingeloggten User müssen sich neu einloggen |
| Asynq-Queue (in-flight Tasks zum Crash-Zeitpunkt) | Nicht mehr verarbeitete Tasks gehen verloren |
| Rate-Limiter-Counter | Werden zurückgesetzt — kein Security-Problem |
| Asynq-Scheduled-Tasks | Werden beim nächsten Scheduler-Tick (≤ 1 Min.) erneut eingestellt |

```bash
docker compose stop redis worker api
docker volume rm vakt_redis_data
docker compose up -d redis
# PONG warten
docker compose up -d worker api
```

---

## Auswirkung auf Asynq-Queue im Detail

- **In-flight Tasks** (bereits an Worker übergeben): Bei ungraceful Shutdown verloren. Kein automatisches Retry (abhängig von `asynq.MaxRetry` pro Task-Typ).
- **Pending Tasks** (noch in Queue): Gehen verloren, wenn Volume gelöscht wird.
- **Scheduled Tasks** (zukünftig geplant): Werden vom Scheduler (`worker`-Prozess) beim nächsten Start neu eingestellt (innerhalb 1 Minute).
- **Unique Guards** (`asynq.Unique`): Schützen gegen Doppel-Enqueueing nach Neustart.

Bei Datenverlust in der Queue: Vakt-Scan-Jobs und Reporting-Jobs können manuell via Admin-UI neu angestoßen werden.

---

## Monitoring

Zabbix-Item `vakt.redis.ping` löst einen Alert aus, wenn Redis nicht erreichbar ist. Die Queue-Tiefe (`asynq.queue.size`) zeigt das Dashboard „Vakt Worker" aus dem **optionalen** Observability-Stack (`docker compose --profile observability up`) — ohne dieses Profil gibt es kein Grafana. Bei wiederholten Ausfällen → HA-Setup prüfen: [`redis-ha.md`](redis-ha.md).

---

*Stand: 2026-07-22*
