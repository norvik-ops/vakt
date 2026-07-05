# Wartungsfenster — Server-Upgrade Strato VC-2-4 → VC-6-12

> Geplanter Wechsel auf größere Strato-vServer-Plattform für `secdemo.norvikops.de` + Landing-Pages.
> Erstellt: 2026-05-20
> Status: Geplant
> Verantwortlich: Stefan Moseler

---

## Warum überhaupt?

Aktuelle Strato VC-2-4 (2 vCPU, 4 GB RAM) ist bei Single-Visitor-Demo OK, aber:

- Ollama qwen2.5:3b braucht im aktiven Context 3.5+ GB → bei 2 parallelen Demo-Visitors Swap, zähflüssiges UX
- Headroom für Spike-Traffic (Hacker News / LinkedIn-Posts) fehlt
- Kein Puffer für späteres Modell-Upgrade auf 7B-Klasse

**Zielsystem VC-6-12** (6 vCPU, 12 GB RAM) gibt Headroom für:
- 3-5 parallele Demo-Visitors mit aktiver AI
- Optionales 7B-Modell (qwen2.5:7b oder gemma2:9b)
- Postgres-Cache spürbar größer

Aufpreis ggü. VC-4-8 ist ~10 €/Monat — Wartungsfenster-Risiko ist die größere Größe.

## Wahl des Zeitfensters

**Empfohlen:** Sonntag, 2026-05-25, 04:00–06:00 UTC (06:00–08:00 MESZ)

- Demo-Traffic ist nach Logs niedrigster Punkt (≈ < 1 Visitor/Stunde)
- LinkedIn-Posts / Outreach laufen Montag-Donnerstag → keine Marketing-Aktion am Sonntag
- Pufferzeit von 2h ist konservativ — realistisch ~ 30-45 min reine Downtime

**Alternativ:** Werktag 23:00–01:00 UTC, wenn am Wochenende ein Termin liegt.

---

## Pre-Flight Checkliste (T-24h)

- [ ] **Backup verifizieren**
  ```bash
  ssh norvikserver "cd /opt/vakt && ./scripts/backup.sh /backups/vakt"
  # Off-Site-Copy:
  rsync -avz norvikserver:/backups/vakt/vakt-backup-*.tar.gz ~/backups/vakt/
  ```
- [ ] **Backup auf einem Test-Container restoren** (oder zumindest `pg_restore --list` durchlaufen lassen)
- [ ] **Image-Tag des aktuell laufenden Containers notieren**
  ```bash
  ssh norvikserver "docker compose -f /opt/vakt/docker-compose.yml ps --format json | jq -r '.[].Image'"
  ```
  → Rollback-Tag merken (z.B. `ghcr.io/matharnica/vakt:v0.14.0`)
- [ ] **Strato-Upgrade-Pfad geklärt**
  - Strato bietet Online-Migration zwischen vServer-Größen via Kundencenter
  - Erwartete Downtime laut Strato-Doku: 5-15 min für Live-Migration, 20-30 min für Backup+Restore-Pfad
  - Falls nur Backup+Restore: IP-Adresse bleibt gleich → DNS unverändert
- [ ] **DNS-TTL prüfen** (falls IP-Wechsel doch erforderlich)
  ```bash
  dig +short secdemo.norvikops.de @1.1.1.1
  dig ns norvikops.de
  ```
  → TTL temporär auf 300s setzen, 24h vorher

## Pre-Flight (T-1h)

- [ ] **Demo-Banner ausrollen** mit Wartungshinweis
  - In `frontend/src/shared/components/DemoBanner.tsx` oder env-Flag `VAKT_DEMO_BANNER_TEXT`
  - Text: „Wartungsarbeiten am 25.05. 04:00-06:00 UTC — Demo eventuell kurz nicht erreichbar."
- [ ] **Health-Snapshot pre-flight**
  ```bash
  curl -s https://secdemo.norvikops.de/health | jq . > /tmp/health-pre.json
  curl -s https://secdemo.norvikops.de/api/v1/version | jq . > /tmp/version-pre.json
  ```
- [ ] **Active-Sessions snapshotten**
  ```bash
  ssh norvikserver "docker exec vakt-postgres psql -U vakt -c 'SELECT COUNT(*) FROM refresh_sessions WHERE expires_at > NOW()'"
  ```

---

## Durchführung (T-Zeitpunkt)

### Variante A: Strato Live-Migration (bevorzugt)

1. **Wartungsmodus aktivieren** — Nginx-Maintenance-Page
   ```bash
   ssh norvikserver "cd /opt/vakt && docker compose -f docker-compose.maintenance.yml up -d"
   ```
   (Maintenance-Compose serviert eine statische HTML-Seite auf :80/:443)
2. **Strato-Kundencenter:** vServer → Upgrade → VC-6-12 wählen → Bestätigen
3. **Warten auf Live-Migration** (im Strato-Dashboard sichtbar)
4. **Nach Reboot:** SSH wieder verfügbar prüfen
   ```bash
   ssh -o ConnectTimeout=10 norvikserver "uname -a && free -h && nproc"
   ```
   → Erwartet: 12 GB RAM, 6 vCPU
5. **Vakt-Stack hochfahren**
   ```bash
   ssh norvikserver "cd /opt/vakt && docker compose down && docker compose up -d"
   ```
6. **Maintenance-Stack stoppen** (wenn Vakt grün)

### Variante B: Backup-Restore-Pfad (Fallback)

Falls Strato Live-Migration nicht anbietet oder fehlschlägt:

1. Vakt stoppen + Backup ziehen (siehe Pre-Flight)
2. Neuen VC-6-12 separat bestellen, IP-Adresse notieren
3. Repo + `.env` + Backup auf neuen Server kopieren
   ```bash
   rsync -avz /opt/vakt/ neuer-server:/opt/vakt/
   ```
4. Migrations + Restore
   ```bash
   ssh neuer-server "cd /opt/vakt && ./scripts/restore.sh /tmp/vakt-backup-*.tar.gz"
   ```
5. DNS-A-Record auf neue IP
   ```bash
   # Strato DNS-Editor oder direkt per API
   ```
6. Alten Server erst nach 48h Beobachtung kündigen

---

## Post-Flight Validierung

Nach dem Upgrade — Reihenfolge wichtig:

- [ ] **Health-Check** matched Pre-Snapshot (außer `version`/`demo`-Felder)
  ```bash
  curl -s https://secdemo.norvikops.de/health | jq . > /tmp/health-post.json
  diff <(jq 'del(.timestamp,.uptime_seconds)' /tmp/health-pre.json) <(jq 'del(.timestamp,.uptime_seconds)' /tmp/health-post.json)
  ```
- [ ] **Demo-Smoke-Test laut [CLAUDE.md `API-Contract-Checks`](../../CLAUDE.md):**
  ```bash
  curl -s https://secdemo.norvikops.de/health | jq '.demo, .sso_enabled, .version'
  DEMO=$(curl -sX POST https://secdemo.norvikops.de/api/v1/demo/start)
  EMAIL=$(echo "$DEMO" | jq -r '.admin_email')
  PASS=$(echo "$DEMO" | jq -r '.admin_password')
  curl -sX POST https://secdemo.norvikops.de/api/v1/auth/login -H 'Content-Type: application/json' \
    -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq '.user | keys'
  ```
- [ ] **AI-Pipeline durchspielen**
  - Login als Demo-User → SecVitals → AI-Agent-Page → Goal `"Liste alle offenen Findings"` → Plan + Tool-Calls laufen
  - Erwartete Response-Zeit: < 8s für ersten Token
- [ ] **Resources passen**
  ```bash
  ssh norvikserver "free -h && nproc && docker stats --no-stream --format '{{.Name}}: {{.MemUsage}} CPU={{.CPUPerc}}'"
  ```
  → Bei Single-User-Test sollte Ollama < 4 GB belegen, Postgres < 200 MB
- [ ] **Asynq-Worker läuft + Periodic-Tasks gescheduled**
  ```bash
  ssh norvikserver "docker logs vakt-worker --tail 50 | grep -E 'scheduler|periodic'"
  ```
- [ ] **Demo-Banner deaktivieren** (Pre-Flight zurückrollen)

## Rollback-Strategie

### Rollback-Trigger
- Login-Smoke-Test ist 30 min nach Up rot
- Migration aus einem 124→125 Schritt fehlgeschlagen
- Ollama startet nicht (Model-Download-Loop)
- Postgres-Daten korrupt (sehr unwahrscheinlich bei Live-Migration)

### Rollback Variante A (Live-Migration → VC-2-4 zurück)
- Im Strato-Kundencenter: vServer-Größe zurück auf VC-2-4 (nur möglich wenn Disk-Größe gleich bleibt — vorher mit Strato-Support klären)
- Backup einspielen wenn Daten betroffen

### Rollback Variante B (neuen Server hatte IP-Wechsel)
- DNS zurück auf alte IP (DNS-TTL = 300s → ≤ 5 min Propagation)
- Alter Server muss noch laufen (in Variante B nicht löschen vor 48h)
- Backup nur einspielen wenn Schreibvorgänge auf neuem Server passiert sind

### Last-Resort
- Backup `/backups/vakt/vakt-backup-2026-05-25_03-30-00.tar.gz` einspielen auf VC-2-4
- Customer-Mitteilung über Status-Page (https://status.norvikops.de — falls noch nicht da, dann via LinkedIn-Post)

---

## Kommunikation

### Vor dem Fenster (T-7d)
- LinkedIn-Post: „Vakt Demo wird am 25.05. kurz für ein Performance-Upgrade pausieren."
- Newsletter (falls Liste > 50 Subscribers existiert)

### Während des Fensters
- Demo-Banner mit Live-Status
- Maintenance-Page mit erwartetem End-Zeitpunkt

### Nach dem Fenster (T+1h)
- LinkedIn-Update mit Resource-Vergleich vor/nach
- Demo-Banner zurück auf default

---

## Lessons-Learned-Vorlage

Nach dem Upgrade hier eintragen:

- Tatsächliche Downtime: ___ min (geplant: 30-60 min)
- Probleme während Migration: ___
- RAM-Auslastung nach 24h Beobachtung: ___ GB (von 12 verfügbar)
- Concurrent-AI-Demos getestet: ___ parallel
- Followup-TODOs: ___

---

## Anhang: Maintenance-Compose

`docker-compose.maintenance.yml` (separat im `ops/`-Branch des Public-Mirror):

```yaml
services:
  maintenance:
    image: nginx:alpine
    ports: ["80:80", "443:443"]
    volumes:
      - ./ops/maintenance.html:/usr/share/nginx/html/index.html:ro
      - /etc/letsencrypt:/etc/letsencrypt:ro
      - ./ops/nginx-maintenance.conf:/etc/nginx/conf.d/default.conf:ro
```

`ops/maintenance.html`: einfache statische Seite mit dem geplanten End-Zeitpunkt.
