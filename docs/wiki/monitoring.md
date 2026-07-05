# Monitoring deiner Vakt-Instanz

Vakt ships with three monitoring surfaces out of the box:
a Prometheus-compatible `/metrics` endpoint, structured JSON logs (zerolog),
and OpenTelemetry distributed traces (opt-in).
This page documents what is real and reachable — not what is merely planned.

---

## Mitgeliefertes Monitoring-Bundle (opt-in)

Vakt liefert einen vollständigen Observability-Stack mit: **Loki** (Logs), **Promtail**
(Log-Collector), **Tempo** (Traces), **Prometheus** (Metrics + Alert-Rules) und
**Grafana** (provisionierte Datasources + Dashboards in `observability/dashboards/`).

```bash
# Bundle zusammen mit dem Haupt-Stack starten (empfohlen):
docker compose -f docker-compose.yml -f docker-compose.observability.yml \
  --profile observability up -d

# Grafana: http://localhost:3000 (admin / Wert aus GRAFANA_ADMIN_PASSWORD, Default: admin)
# Provisionierte Datasources: Loki, Tempo, Prometheus
# Provisionierte Dashboards: API-Overview, Worker, Demo-Metrics
```

Voraussetzung: `VAKT_METRICS_TOKEN` in `.env` setzen, damit Prometheus `/metrics` mit
Bearer-Auth scrapen kann (siehe Abschnitt "Prometheus Metrics" unten).

Alternativ dauerhaft via `COMPOSE_FILE`:
```bash
echo 'COMPOSE_FILE=docker-compose.yml:docker-compose.observability.yml' >> .env
docker compose --profile observability up -d
```

---

## Health Endpoints

Both the **API** (`:8080` / behind nginx `:80`) and the **Worker** (`:9090`) expose health routes.

### API

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /health` | None | Liveness — always returns `{"status":"ok"}` while the process is up |
| `GET /health/ready` | None | Readiness — checks DB + Redis; returns 503 if either is down |

```bash
# Check API liveness (via nginx)
curl -s http://localhost/health | jq .

# Check API readiness (directly, skipping nginx)
curl -s http://localhost:8080/health/ready | jq .
```

### Worker

The worker runs a lightweight HTTP health server on **`:9090`** (Docker-internal only, not exposed via nginx).

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /health` | None | Liveness |
| `GET /health/ready` | None | Readiness — checks DB + Redis |
| `GET /health/queue` | None | JSON snapshot of all Asynq queue depths and stats |

```bash
# From within the Docker network (e.g. from the api container)
curl -s http://vakt-worker:9090/health/queue | jq .
```

---

## Prometheus Metrics

### Access

`/metrics` is served by the API on port `:8080` and is **IP-allowlisted** to localhost
and Docker-internal subnets (`172.16.0.0/12`, `10.0.0.0/8`, `192.168.0.0/16`).
It is **not exposed via nginx** to the public internet — scraping must happen from within
the Docker network or from the host.

```bash
# From the host (Docker-internal access via mapped port)
curl -s http://localhost:8080/metrics

# With optional Bearer token (set VAKT_METRICS_TOKEN to restrict access)
curl -s -H "Authorization: Bearer $VAKT_METRICS_TOKEN" http://localhost:8080/metrics
```

### Configuration

Metrics are **enabled by default**.

```env
# To disable metrics (e.g. on resource-constrained instances):
VAKT_METRICS_DISABLED=true

# Optional: require a Bearer token for /metrics (still IP-allowlisted)
VAKT_METRICS_TOKEN=<random-secret>
```

### Real Metric Names

These are all metrics emitted by a running Vakt instance (verified against `metrics/handler.go`):

| Metric | Type | Description |
|---|---|---|
| `vakt_findings_total{severity}` | gauge | Open findings by severity (critical/high/medium/low) |
| `vakt_score_current` | gauge | Current aggregate security score (0–100) |
| `vakt_dsr_open_total` | gauge | Open data subject requests |
| `vakt_dsr_overdue_total` | gauge | Overdue DSRs (past `due_date`) |
| `vakt_backup_age_hours` | gauge | Hours since last recorded backup (999 = never) |
| `vakt_organizations_total` | gauge | Total organisations on this instance |
| `vakt_active_sessions_total` | gauge | Active user sessions |
| `vakt_db_pool_in_use` | gauge | pgxpool connections currently checked out |
| `vakt_db_pool_idle` | gauge | pgxpool idle connections |
| `vakt_queue_depth{queue}` | gauge | Asynq pending + active jobs per queue |
| `vakt_open_risks_total{org_id}` | gauge | Open risks per organisation |
| `vakt_open_capas_total{org_id}` | gauge | Open or in-progress CAPAs per organisation |
| `vakt_overdue_capas_total{org_id}` | gauge | Overdue CAPAs per organisation |
| `vakt_open_incidents_total{org_id}` | gauge | Open security incidents per organisation |
| `vakt_controls_total{org_id,framework_id}` | gauge | Total controls per org and framework |
| `vakt_controls_implemented{org_id,framework_id}` | gauge | Implemented controls per org and framework |
| `vakt_asynq_jobs_total{task_type,result}` | counter | Background job completions by type and result (ok/error) |
| `vakt_asynq_jobs_duration_ms_sum{task_type}` | gauge | Sum of job durations (ms) per task type |
| `vakt_asynq_jobs_duration_ms_max{task_type}` | gauge | Max job duration (ms) per task type |

### Prometheus Scrape Configuration

Scrape from within the Docker network (e.g. with a Prometheus container in the same
compose network or via `host.docker.internal:8080` from a co-located Prometheus):

```yaml
# prometheus.yml
scrape_configs:
  - job_name: vakt-api
    static_configs:
      - targets: ["vakt-api:8080"]   # Docker service name
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials: "<VAKT_METRICS_TOKEN>"
    scrape_interval: 60s

  # Worker metrics — exposes vakt_worker_up gauge (1=healthy, 0=degraded)
  - job_name: vakt-worker
    static_configs:
      - targets: ["vakt-worker:9090"]
    metrics_path: /metrics
    scrape_interval: 30s
```

---

## OpenTelemetry Traces (opt-in)

Distributed traces (spans) are exported via OTLP/HTTP when
`OTEL_EXPORTER_OTLP_ENDPOINT` is set. If the variable is absent, tracing is
a no-op — no traces leave the instance.

```env
# Export to a local Tempo/Jaeger/etc instance
OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318

# Optional: add auth headers (e.g. for Grafana Cloud Tempo)
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic <base64>
```

HTTP handlers are instrumented via `otelecho.Middleware`; Asynq handlers get spans
automatically. The service name in traces is `vakt-api`.

---

## Structured Logs

All log output is JSON (zerolog) to stdout. Example:

```json
{"level":"info","ts":"2026-06-01T08:00:01Z","module":"vaktscan","msg":"scan completed","org_id":"...","findings":12}
```

**Log level** is set via `VAKT_LOG_LEVEL` (default `info`). Valid values: `trace`, `debug`, `info`, `warn`, `error`.

For log aggregation (Loki, ELK, CloudWatch): use `docker compose logs --follow` to tail
or mount a log driver in `docker-compose.yml`. Logs are written to stdout only (no files).

---

## Recommended Alert Rules

These expressions are validated against real metric names. Add them to `prometheus/rules/vakt.yml`:

```yaml
groups:
  - name: vakt
    rules:

      # API not reachable
      - alert: VaktAPIDown
        expr: up{job="vakt-api"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Vakt API not reachable"
          description: "The /metrics endpoint has been unreachable for 2 minutes."

      # DB connection pool near exhaustion (default pool size: 25)
      - alert: VaktDBPoolHigh
        expr: vakt_db_pool_in_use > 20
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Vakt DB connection pool > 80% used"
          description: "{{ $value }} of ~25 pool connections are in use."

      # Asynq job queue backing up
      - alert: VaktQueueDepthHigh
        expr: vakt_queue_depth{queue="default"} > 200
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Vakt default queue depth high"
          description: "{{ $value }} jobs pending in the default queue."

      # Backup overdue (no backup recorded in 26 hours)
      - alert: VaktBackupOverdue
        expr: vakt_backup_age_hours > 26
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Vakt backup overdue"
          description: "Last backup was {{ $value | printf \"%.0f\" }} hours ago."

      # Critical findings present
      - alert: VaktCriticalFindings
        expr: vakt_findings_total{severity="critical"} > 0
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "{{ $value }} critical findings in Vakt"
```

---

## Sizing & HA Reference

### Single-Node (recommended for self-hosted)

Vakt runs as a single-node stack by design. All state is in PostgreSQL and Redis;
the API and Worker are stateless and can be restarted freely.

| Component | RAM | Notes |
|---|---|---|
| vakt-api | 128–256 MB | Stateless; scales horizontally (Redis-backed sessions) |
| vakt-worker | ~200 MB | Horizontal skalierbar via `WORKER_REPLICAS`. Asynq garantiert exakt-einmal-Ausführung. |
| vakt-frontend | 32–64 MB | Static nginx; scales freely |
| PostgreSQL | 512 MB–2 GB | Primary bottleneck; monitor with `pg_stat_activity` |
| Redis | 64–256 MB | Queue + session store; persistence optional |
| Ollama (AI, opt-in) | 5–8 GB | `qwen2.5:7b` default (~4.5 GB live RAM, needs 8 GB; `qwen2.5:3b` ~1.9 GB on small VMs); disable with `VAKT_AI_PROVIDER=disabled` |
| **Total (without Ollama)** | **~1 GB** | Fits on a 2 GB VPS |
| **Total (with Ollama)** | **~4 GB** | Hetzner CX22 minimum; CX32 (8 GB) recommended |

### pgbouncer

Docker Compose includes `pgbouncer` as a connection pool sidecar.
The Helm chart does **not** include pgbouncer — use an external pgbouncer Helm chart
or tune `VAKT_DB_MAX_CONNS` to match your PostgreSQL `max_connections`.

### High Availability

| Component | HA Strategy |
|---|---|
| API | Multiple replicas (stateless, Redis-backed) |
| Worker | Horizontal scaling via `WORKER_REPLICAS`. Asynq (Redis-backed) ensures tasks run exactly once. See "Worker skalieren" below. |
| PostgreSQL | Streaming replication or managed DB (RDS, Hetzner Managed PostgreSQL) |
| Redis | Redis Sentinel or managed Redis |

---

## Worker skalieren

Der Worker ist zustandslos — alle Jobs werden über Redis/Asynq koordiniert.
Asynq garantiert, dass ein Task **exakt einmal** ausgeführt wird, auch wenn mehrere
Worker-Replicas gleichzeitig laufen.

### Schnellstart

```bash
# 3 Worker-Replicas starten
WORKER_REPLICAS=3 docker compose up -d

# Status prüfen
docker compose ps worker
```

### Kapazitätsplanung

| Parameter | Formel | Beispiel (3 Replicas) |
|---|---|---|
| Parallele Tasks gesamt | `VAKT_WORKER_CONCURRENCY` × `WORKER_REPLICAS` | 8 × 3 = 24 |
| Redis `maxclients` (Minimum) | `VAKT_WORKER_CONCURRENCY` × `WORKER_REPLICAS` + Reserve | 24 + 10 = 34 |
| RAM-Bedarf (Worker) | ~200 MB × `WORKER_REPLICAS` | ~600 MB |

Redis `maxclients` lässt sich prüfen mit:
```bash
docker compose exec redis redis-cli CONFIG GET maxclients
```

### Hinweise

- `WORKER_HEALTH_PORT` (Standard `9090`) muss pro Replica eindeutig sein, wenn mehrere
  Worker auf demselben Host mit gemappten Ports laufen. Im Docker-Netzwerk ist das
  kein Problem — jede Replica bekommt eine eigene IP.
- Die Prometheus-Scrape-Config in `monitoring.md` scrapt alle Replicas automatisch,
  wenn der `vakt-worker`-DNS-Name auf mehrere Container auflöst.
- Für `docker compose` (ohne Swarm) ist `deploy.replicas` ab Compose Spec v3.4
  unterstützt.
