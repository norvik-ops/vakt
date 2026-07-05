# Operator Runbook — Vakt

Zielgruppe: Systemadministratoren selbst gehosteter Vakt-Instanzen.

---

## 1. Production-Deployment-Checkliste

Vor dem ersten Produktiv-Start oder nach einem Bare-Metal-Neuaufbau:

### Pflicht-Checks

- [ ] **VAKT_SECRET_KEY gesetzt** — mindestens 32 Hex-Zeichen (64 Zeichen empfohlen):
  ```bash
  grep VAKT_SECRET_KEY .env | cut -d= -f2 | wc -c
  # Ausgabe muss > 64 sein (inkl. Newline)
  # Generieren: openssl rand -hex 32
  ```
  Wenn Key leer oder zu kurz: API-Container startet nicht (Fatal-Log).

- [ ] **TLS konfiguriert** — Caddy (empfohlen) oder eigener Reverse-Proxy mit gültigem Zertifikat:
  ```bash
  # Mit Caddy (automatisches Let's Encrypt):
  # Caddyfile: vakt.example.com { reverse_proxy localhost:80 }
  caddy start

  # Oder: eigener nginx/traefik vor dem Vakt-nginx-Container
  ```

- [ ] **HTTPS-Zertifikat gültig** — Ablauf > 30 Tage:
  ```bash
  echo | openssl s_client -connect vakt.example.com:443 2>/dev/null \
    | openssl x509 -noout -dates
  ```

- [ ] **Backup-Cron aktiv** — täglicher pg_dump (02:00 Uhr empfohlen):
  ```cron
  0 2 * * * cd /opt/vakt && ./scripts/backup.sh /backups/vakt >> /var/log/vakt-backup.log 2>&1
  ```
  Dokumentation: [`docs/operations/backup-restore.md`](backup-restore.md)

- [ ] **Update-Cron konfiguriert** — Watchtower oder manueller Pull-Cron:
  ```bash
  # Watchtower (automatische Image-Updates):
  docker run -d --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    containrrr/watchtower --interval 86400

  # Oder: manueller wöchentlicher Update-Cron
  # 0 3 * * 0 cd /opt/vakt && docker compose pull && docker compose up -d
  ```

- [ ] **Firewall** — Port 80/443 offen, Port 5432/6379 nicht von außen erreichbar
- [ ] **Redis-Passwort gesetzt** — `VAKT_REDIS_URL` enthält Passwort; `requirepass` in `docker-compose.yml` aktiv
- [ ] **SMTP konfiguriert** — wenn Vakt Aware (Phishing-Kampagnen) genutzt wird: `VAKT_SMTP_*` setzen

- [ ] **Monitoring-Bundle aktiviert** (empfohlen) — optionaler Loki/Tempo/Prometheus/Grafana-Stack:
  ```bash
  docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability up -d
  ```
  Vollständige Anleitung: [`docs/wiki/monitoring.md`](../wiki/monitoring.md)

---

## 2. Häufige Probleme

> **Restore / Datenverlust / Host-Verlust:** Für diese Szenarien → [`docs/runbooks/disaster-recovery.md`](../runbooks/disaster-recovery.md). Dieser Abschnitt deckt nur Fix-in-place-Szenarien ohne Restore.

---

### Disk voll — Postgres geht read-only / Container crashen

**Symptom:** Container crashen mit `No space left on device`; PostgreSQL antwortet mit
`ERROR: could not write to file` oder geht in Read-Only-Modus; API gibt 500 für Schreiboperationen.

**Diagnose:**
```bash
# Freien Speicher auf dem Host prüfen
df -h

# Größten Speicherfresser finden (Docker-Volumes)
du -sh /var/lib/docker/volumes/*

# Docker-Log-Größe prüfen (häufig unterschätzt)
du -sh /var/lib/docker/containers/*/

# Dangling Images und Build-Cache
docker system df
```

**Lösung:**
```bash
# 1. Sofort: Alten Build-Cache + Dangling Images entfernen (sicher, kein Datenverlust)
docker image prune -f
docker builder prune -f

# 2. Alte Logs truncaten (Datei bleibt, Inhalt wird geleert — sicher während Container läuft)
truncate -s 0 /var/lib/docker/containers/<container-id>/*-json.log

# 3. Alte Backups aufräumen (Retention prüfen: standardmäßig 7 Tage)
ls -lh /backups/vakt/
find /backups/vakt/ -name "*.tar.gz" -mtime +7 -delete

# 4. Nach Bereinigung: Postgres neu starten
docker compose restart postgres vakt-api
```

**Prävention:**
- Docker Log-Rotation konfigurieren in `/etc/docker/daemon.json`:
  ```json
  { "log-driver": "json-file", "log-opts": { "max-size": "50m", "max-file": "3" } }
  ```
- Disk-Auslastungs-Alert bei > 80 %: Zabbix-Item `vfs.fs.size[/,pused]` oder Prometheus `node_filesystem_avail_bytes`.

---

### Last-Spike / Performance-Einbruch

**Symptom:** Hohe API-Latenzen, `/health/ready` gibt `degraded` zurück, Requests timeouten.

**Diagnose:**
```bash
# RED-Metriken (Rate, Errors, Duration) über /metrics
curl -s -H "Authorization: Bearer $VAKT_METRICS_TOKEN" http://localhost:8080/metrics \
  | grep -E "vakt_queue_depth|vakt_db_pool"

# Aktive DB-Connections
docker compose exec postgres psql -U vakt -c \
  "SELECT state, count(*) FROM pg_stat_activity WHERE datname='vakt' GROUP BY state;"

# Langsame Queries (letzte 10 Minuten, sortiert nach Total-Zeit)
docker compose exec postgres psql -U vakt -c \
  "SELECT query, calls, total_exec_time::int/1000 AS total_s, mean_exec_time::int AS mean_ms
   FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 10;"

# Worker-Queue-Tiefe
curl -s http://localhost:9090/health/queue | jq '{pending: .queues[].pending, active: .queues[].active}'
```

**Lösung:**
```bash
# Kurzfristig: API-Container neu starten (beendet blockierte Goroutinen)
docker compose restart vakt-api

# Wenn DB-Pool erschöpft (vakt_db_pool_in_use > 12):
docker compose exec postgres psql -U vakt -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity
   WHERE datname='vakt' AND state='idle' AND query_start < NOW() - INTERVAL '5 min';"
```

**Langfristig:** Für Profiling → [Performance-Profiling (pprof)](#pprof). Für Skalierung → [`docs/operations/scaling.md`](scaling.md).

---

### Queue-Backlog / Worker hängt

**Symptom:** Asynq-Jobs stauen sich (E-Mails/Evidence-Sync verzögert), Worker-Logs zeigen
keine neuen Jobs, `vakt_queue_depth` in Prometheus steigt.

**Diagnose:**
```bash
# Queue-Snapshot vom Worker-Health-Endpoint
curl -s http://localhost:9090/health/queue | jq .

# Worker-Logs auf Fehler prüfen
docker compose logs vakt-worker --tail=50 | grep '"level":"error"'

# Redis erreichbar?
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" ping
```

**Was der Queue-Snapshot bedeutet:**
- `pending` > 0 + `active` = 0 → Worker läuft nicht oder hängt
- `retry` > 10 → wiederholte Job-Fehler (Fehlermeldung im Worker-Log lesen)
- `archived` wächst → permanente Fehler, Jobs werden nicht mehr retried

**Lösung:**
```bash
# Worker neu starten (Redis-Persistenz hält Jobs — kein Job-Verlust)
docker compose restart vakt-worker

# Vergifteten Job-Typ identifizieren und archivierte Jobs löschen (Asynq-CLI)
# Im Container:
docker compose exec vakt-worker sh -c 'asynq queue ls'

# Wenn ein bestimmter Job-Typ dauerhaft failt: Asynq-Queue dieses Typs leeren
# (nur wenn klar ist, dass die Jobs nicht mehr relevant sind)
docker compose exec vakt-worker sh -c 'asynq task ls --queue default --state archived'
```

**Prävention:** Alert-Rule `VaktQueueDepthHigh` in `observability/alert-rules.yaml` ist bereits
vorkonfiguriert (Warning bei > 200 pending Jobs über 10 min).

---

### "DB Connection refused" beim Start

**Symptom:** API-Container-Logs zeigen `connection refused` oder `dial tcp ... connect: connection refused`

**Ursache:** PostgreSQL noch nicht vollständig bereit, als API-Container startet.

**Lösung:**
```bash
# PostgreSQL-Status prüfen
docker compose ps postgres
docker compose logs postgres | tail -20

# Wenn postgres healthy ist, API neu starten:
docker compose restart vakt-api

# Falls postgres nicht startet — Volume-Problem?
docker compose down
docker compose up -d postgres
# Auf "database system is ready to accept connections" warten
docker compose up -d
```

**Langfristig:** Der `healthcheck` in `docker-compose.yml` stellt sicher, dass `api` erst startet, wenn `postgres` healthy ist. Bei Problemen `depends_on.postgres.condition: service_healthy` prüfen.

---

### "Redis NOAUTH" oder "WRONGPASS"

**Symptom:** Logs zeigen `NOAUTH Authentication required` oder `WRONGPASS`

**Ursache:** Redis ist mit Passwort gestartet, aber `VAKT_REDIS_URL` enthält kein oder falsches Passwort.

**Lösung:**
```bash
# Aktuelles Redis-Passwort prüfen:
grep requirepass docker-compose.yml

# VAKT_REDIS_URL in .env anpassen — Format:
# VAKT_REDIS_URL=redis://:PASSWORT@redis:6379

# Stack neu starten:
docker compose restart vakt-api vakt-worker
```

---

### "AI model download stuck"

**Symptom:** Worker-Logs zeigen Ollama-Pull-Meldungen, AI-Features antworten nicht

**Ursache:** Der `ollama-init`-Container zieht das Standard-Modell (`qwen2.5:7b`, ~4.5 GB). Das ist kein Fehler — je nach Bandbreite dauert das **3–30 Minuten**.

**Diagnose:**
```bash
docker compose logs ollama-init --follow
docker compose logs vakt-worker | grep -i ollama
```

**Erwartete Ausgabe:**
```
pulling qwen2.5:3b... 1.2 GB / 1.9 GB
```

**Lösung:** Warten. Die Plattform läuft ohne AI-Features währenddessen normal. AI-Endpoints geben einen `503`-Fehler zurück bis das Modell verfügbar ist.

**Falls Download steckengeblieben ist (> 60 min ohne Fortschritt):**
```bash
docker compose restart ollama ollama-init
```

---

### "Demo-Login zeigt Fehler" / Demo-Credentials funktionieren nicht

**Symptom:** Login-Screen im Demo-Modus zeigt Fehler oder keine Credentials

**Ursache in 95% der Fälle:** Demo-Endpoint antwortet nicht oder Worker läuft nicht.

**Diagnose:**
```bash
# Schritt 1: Demo-Endpoint direkt testen
curl -sX POST http://localhost/api/v1/demo/start | jq .

# Schritt 2: Wenn kein 200 — Worker-Container prüfen
docker compose ps vakt-worker
docker compose logs vakt-worker | tail -20

# Schritt 3: API-Container prüfen (Migrations-Status)
docker compose logs vakt-api | grep -E "migration|error|fatal"
```

**Häufige Ursachen:**
- Worker läuft nicht → `docker compose restart vakt-worker`
- Migrations fehlgeschlagen → `docker compose logs migrate` prüfen
- `VAKT_DEMO=true` nicht gesetzt → `.env` prüfen
- PostgreSQL-Connection-Problem → siehe "DB Connection refused"

> Demo-Login-Credentials sind **niemals statisch** — sie werden pro Session ephemer generiert und laufen nach 4 Stunden ab. `admin@vakt.local / admin1234` o.ä. sind keine gültigen Credentials.

---

### API antwortet mit 500 für alle Requests

**Symptom:** Alle API-Endpoints geben HTTP 500 zurück

**Diagnose:**
```bash
docker compose logs vakt-api | tail -50
# Auf "level":"error" oder "level":"fatal" achten
```

**Häufige Ursachen:**
- Fehlende oder falsche `VAKT_SECRET_KEY` → API startet nicht vollständig
- Datenbank-Schema nicht aktuell → `docker compose run --rm migrate up`
- Redis nicht erreichbar → Fallback aktiv, aber Session-Features eingeschränkt

---

### Hohe Memory-Nutzung / OOM-Kills

**Symptom:** Container werden vom OOM-Killer beendet

**Diagnose:**
```bash
docker stats
# Ollama-Container nutzt ~1.9 GB für qwen2.5:3b + Inference-Overhead
```

**Lösung:**
- Minimum: 4 GB RAM (ohne AI: 2 GB)
- Mit AI-Modell: 4 GB RAM + 2 GB für Modell
- Ollama deaktivieren wenn RAM knapp: `VAKT_AI_PROVIDER=disabled` in `.env` + `ollama`/`ollama-init` Services aus Compose entfernen

<a id="pprof"></a>
### Performance-Profiling (pprof)

**Wann:** Memory-Leak-Verdacht über Tage/Wochen, CPU-Hotpath, Goroutine-Leck.

pprof ist **standardmäßig aus**. Aktivieren via `VAKT_PPROF_ENABLED=true` (Container neu starten).
Der pprof-Server lauscht **nur** auf `127.0.0.1:6060` — von außen nicht erreichbar. Zugriff daher
über einen SSH-Tunnel bzw. direkt auf dem Host:

```bash
# Heap-Snapshot (idle-/Leak-Analyse)
go tool pprof http://127.0.0.1:6060/debug/pprof/heap

# CPU-Profil über 30 s unter Last
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30

# Goroutine-Dump (Leak-Suche)
curl -s http://127.0.0.1:6060/debug/pprof/goroutine?debug=1 | head -40

# Im Container ohne lokales go-Toolchain: Profil-Datei ziehen und lokal öffnen
curl -s http://127.0.0.1:6060/debug/pprof/heap -o heap.pb.gz
go tool pprof -http=:8081 heap.pb.gz   # interaktive Web-UI
```

Nach der Diagnose `VAKT_PPROF_ENABLED` wieder auf `false` setzen.

### audit_log-Partitionen & Aufbewahrung (S98-10)

`audit_log` ist jahresweise partitioniert. Ein monatlicher Worker-Job
(`audit:partition_maint`, 1. des Monats 03:30 UTC) legt die Partitionen für das
laufende + die nächsten zwei Jahre an und **droppt** Partitionen, die älter als
`VAKT_AUDIT_RETENTION_YEARS` (Default 6) sind. Eine `DEFAULT`-Partition fängt
alles außerhalb der Bereiche, sodass Inserts nie fehlschlagen.

```bash
# Aktuelle Partitionen anzeigen
docker compose exec postgres psql -U vakt -c \
  "SELECT inhrelid::regclass FROM pg_inherits WHERE inhparent='audit_log'::regclass ORDER BY 1;"
```

**Vor dem Droppen archivieren** (wenn gesetzlich/forensisch nötig): die alte
Partition vor dem Job-Lauf sichern, dann erst verfallen lassen:

```bash
docker compose exec postgres pg_dump -U vakt -t audit_log_2020 vakt > audit_log_2020.sql
```

Wer gar nicht droppen will: `VAKT_AUDIT_RETENTION_YEARS=0` (Vorab-Anlage läuft
weiter, nichts wird entfernt).

---

## 3. Log-Interpretation

Vakt verwendet **zerolog** für strukturiertes JSON-Logging. Jede Zeile ist ein JSON-Objekt.

### Format

```json
{"level":"info","time":"2026-05-24T10:23:45Z","caller":"handler.go:142","msg":"request","method":"POST","path":"/api/v1/vaktcomply/controls","status":201,"latency_ms":12}
```

### Wichtige Felder

| Feld | Bedeutung |
|---|---|
| `level` | `trace`, `debug`, `info`, `warn`, `error`, `fatal` |
| `time` | RFC3339 UTC |
| `caller` | Datei + Zeile |
| `msg` | Kurztext |
| `error` | Fehlermeldung (bei level error/fatal) |
| `org_id` | Organisation (bei auth. Requests) |
| `user_id` | User (bei auth. Requests) |
| `job_type` | Asynq-Job-Typ (bei Worker-Logs) |

### Log-Level-Bedeutung

| Level | Bedeutung | Aktion nötig? |
|---|---|---|
| `info` | Normalbetrieb | Nein |
| `warn` | Unerwarteter Zustand, aber handled | Beobachten |
| `error` | Fehler aufgetreten, Request fehlgeschlagen | Ja — untersuchen |
| `fatal` | Kritischer Fehler, Prozess beendet | Sofort — Container startet nicht neu ohne Fix |

### Nützliche Log-Befehle

```bash
# Nur Fehler anzeigen:
docker compose logs vakt-api 2>&1 | grep '"level":"error"'

# Worker-Job-Fehler:
docker compose logs vakt-worker 2>&1 | grep '"level":"error"'

# Alle Requests mit Latenzen > 1s (simple grep):
docker compose logs vakt-api 2>&1 | grep '"latency_ms"' | \
  python3 -c "import sys,json; [print(l) for l in sys.stdin if json.loads(l).get('latency_ms',0)>1000]"
```

---

## 4. Upgrade-Prozedur

Vor jedem Upgrade: Backup erstellen.

```bash
# Standard-Upgrade
./scripts/backup.sh /backups/vakt
docker compose pull
docker compose run --rm migrate up
docker compose up -d
```

Vollständige Anleitung: [`docs/operations/upgrade.md`](upgrade.md)

---

## 5. Notfall-Kontakte

- GitHub Issues (öffentlich): https://github.com/norvik-ops/vakt/issues
- Security-Meldungen: security@norvikops.de (nicht öffentlich melden)
- Dokumentation: vakt.norvikops.de/docs
