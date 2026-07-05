# Vakt — Deployment Guide

> Selbst-gehostete Security & Compliance Dokumentationsplattform. Alle Daten bleiben in deiner eigenen Infrastruktur.

---

## 1. Voraussetzungen

### Software

Vakt benötigt **Docker Engine 24+** und **Docker Compose v2**.

> **Wichtig: Compose v1 vs. v2**
> Docker Compose v1 war ein separates Python-Tool (`docker-compose`). Seit Docker Desktop 3.6 und Docker Engine 23+ ist Compose v2 direkt in Docker integriert und wird als `docker compose` (ohne Bindestrich) aufgerufen. Stelle sicher, dass du die neue Version verwendest:
> ```bash
> docker compose version   # sollte "Docker Compose version v2.x.x" ausgeben
> ```

**Empfohlene Betriebssysteme:**
- Ubuntu 22.04 LTS (empfohlen)
- Debian 12 (Bookworm)
- Rocky Linux 9 / AlmaLinux 9

### Netzwerk

- Port **80** (HTTP) und **443** (HTTPS) müssen von außen erreichbar sein
- Eine öffentliche IP-Adresse oder ein DNS-Eintrag für HTTPS

### Systemanforderungen

| Ressource | Minimum | Empfohlen | Mit KI-Berater (Standard) |
|---|---|---|---|
| CPU | 2 Kerne | 4 Kerne | 4 Kerne (kein GPU nötig) |
| RAM | 2 GB | 4 GB | 8 GB (für qwen2.5:7b) |
| Disk | 20 GB SSD | 40 GB SSD | 40 GB SSD + ~5 GB für das KI-Modell |
| Betriebssystem | Linux 64-bit | Ubuntu 22.04 LTS | Ubuntu 22.04 LTS |

> **Hinweis:** Der KI-Berater läuft **standardmäßig** lokal via Ollama auf der CPU — kein GPU, kein Cloud-API-Key nötig. Das Default-Modell `qwen2.5:7b` (~4.5 GB) wird beim ersten `docker compose up` automatisch geladen. Deaktivieren: `VAKT_AI_PROVIDER=disabled` in `.env`. Details: [Abschnitt 9](#9-ki-compliance-berater-konfigurieren).

---

## 2. Schnellstart (5 Minuten)

```bash
# 1. Docker installieren (Ubuntu / Debian)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # Neuanmeldung danach erforderlich

# 2. Vakt klonen
git clone https://github.com/norvik-ops/vakt
cd vakt

# 3. Konfiguration
cp .env.example .env

# Secret Key + Passwörter generieren und automatisch eintragen:
sed -i "s/^VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY=$(openssl rand -hex 32)/" .env
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env
sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$(openssl rand -hex 24)/" .env

# 4. Starten
docker compose up -d

# 5. Status prüfen (alle Container sollten "healthy" sein)
docker compose ps
```

Nach dem Start ist Vakt erreichbar unter:

- **http://deine-server-ip** (nach HTTPS-Einrichtung: https://deine-domain.com)

Wenn `VAKT_DEMO=true` gesetzt wurde, legt die Login-Seite beim ersten Aufruf automatisch eine eigene ephemere Demo-Organisation an und zeigt die generierten Zugangsdaten (Admin + Analyst) direkt im Login-Formular an. Die Passwörter sind 16-stellige Zufallswerte; jede Demo-Session lebt 4 Stunden und wird dann automatisch gelöscht. Details: [Demo-Modus](wiki/demo-mode.md).

> **Hinweis:** Demo-Modus **niemals** in Produktionsumgebungen mit echten Daten aktivieren.

---

## 3. Produktions-Deployment

### HTTPS (automatisch via Caddy)

Der Stack enthält **Caddy als Frontdoor** — es holt und erneuert Let's-Encrypt-Zertifikate **vollautomatisch**, ohne Certbot oder Cronjobs. Du musst nur deine Domain setzen:

```bash
# In .env:
VAKT_DOMAIN=vakt.example.com
```

Danach `docker compose up -d`. Caddy terminiert HTTPS auf Port 443 und leitet HTTP (Port 80) automatisch dorthin um. Voraussetzung: Ports **80 und 443** sind aus dem Internet erreichbar (für die ACME-Domain-Validierung) und die Domain zeigt per DNS auf den Server.

Ohne `VAKT_DOMAIN` (Default `localhost`) serviert Caddy HTTPS mit einem lokal signierten Zertifikat — praktisch für Tests. Für den Betrieb hinter einem eigenen TLS-Terminator: `VAKT_DOMAIN=:80` setzen, dann serviert Caddy nur HTTP. Die Routing-Regeln stehen im `Caddyfile` im Projektverzeichnis.

### Firewall einrichten (ufw)

```bash
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP (wird von Caddy zu HTTPS weitergeleitet)
ufw allow 443/tcp   # HTTPS
ufw enable
ufw status
```

---

## 4. Datenbankmigrationen

Vakt verwendet [golang-migrate](https://github.com/golang-migrate/migrate) für versionierte Datenbankmigrationen.

**Migrationen laufen automatisch** — ein dedizierter `migrate`-Container führt alle ausstehenden Migrationen aus, bevor `api` und `worker` starten. Die Startreihenfolge wird über `depends_on` in `docker-compose.yml` erzwungen: `api` und `worker` warten, bis `migrate` erfolgreich abgeschlossen ist.

Es ist kein manuelles Eingreifen und keine `AUTO_MIGRATE`-Umgebungsvariable erforderlich.

**Manuelle Migration** (z. B. um den Zeitpunkt bei größeren Updates selbst zu kontrollieren):

```bash
# Backup zuerst (immer empfohlen)
docker compose exec postgres pg_dump -U vakt vakt > backup_pre_migration_$(date +%Y%m%d).sql

# Migration manuell ausführen
docker compose run --rm migrate
```

> Vor jeder Migration ein Backup anlegen. Nach einem fehlgeschlagenen Update lässt sich so der Ausgangszustand wiederherstellen.

> **Erstmaliger Einsatz der neuen migrate-Service-Konfiguration:** Wer noch eine ältere `docker-compose.yml` ohne den `migrate`-Service verwendet, führt einmalig `docker compose run --rm migrate` manuell aus und startet danach mit `docker compose up -d`.

---

## 5. Backups

### Manuelles Backup

```bash
# PostgreSQL-Dump erstellen
docker compose exec postgres pg_dump -U vakt vakt > backup_$(date +%Y%m%d).sql

# Backup einspielen (Restore)
cat backup_20260511.sql | docker compose exec -T postgres psql -U vakt vakt
```

### Automatisches tägliches Backup (crontab)

```bash
# crontab -e
0 2 * * * cd /opt/vakt && docker compose exec postgres pg_dump -U vakt vakt > /backups/backup_$(date +\%Y\%m\%d).sql
```

Backups am besten auf einem separaten Volume oder externen Speicher ablegen.

---

## 6. Update-Benachrichtigungen

Vakt prüft **nicht automatisch** auf Updates (kein Phone-Home). Es gibt zwei opt-in Mechanismen:

### Option 1 — In-App-Banner (empfohlen)

Aktiviere in deiner `.env`:
```
VAKT_UPDATE_CHECK=true
```

Vakt prüft einmal täglich gegen die [GitHub Releases API](https://github.com/norvik-ops/vakt/releases), ob eine neuere Version verfügbar ist. Administratoren und Eigentümer sehen dann einen Hinweis-Banner in der Oberfläche. Dabei werden **keine Daten gesendet** — es ist ein einfacher GET-Request gegen die öffentliche GitHub-API, ohne Instanz-ID oder sonstige Informationen.

### Option 2 — Watchtower (automatische Updates)

Watchtower aktualisiert Docker-Container automatisch, wenn ein neues Image verfügbar ist. Aktiviere es in `docker-compose.yml`:

```yaml
watchtower:
  image: containrrr/watchtower:latest
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
  command: --schedule "0 0 3 * * *" --cleanup api worker
  restart: unless-stopped
```

> Nur für nicht-kritische Instanzen oder wenn du den Update-Pfad getestet hast.

### Manuelle Updates

```bash
docker compose pull
docker compose up -d
```

---

## 7. Updates (manuell / automatisch)

Hier ist beschrieben, was zu tun ist, wenn ein neues Feature oder eine neue Version ausgerollt werden soll.

---

### Automatisch (wenn Watchtower läuft)

Watchtower holt nächtlich neue Images von GHCR, startet betroffene Container neu und der `migrate`-Service läuft dabei automatisch zuerst. Es ist nichts weiter zu tun — nur die Logs gelegentlich prüfen:

```bash
docker compose logs migrate
docker compose ps
```

---

### Manuell / nach einem eigenen Build

1. **Neuesten Stand holen:**
   ```bash
   git pull
   ```

2. **Images aktualisieren** — eine der beiden Varianten:
   ```bash
   docker compose build          # selbst bauen (für lokale Änderungen)
   # ODER
   docker compose pull           # fertige Images von GHCR holen
   ```

3. **Dienste neu starten:**
   ```bash
   docker compose up -d
   ```
   `docker compose up -d` startet den `migrate`-Container automatisch zuerst — `api` und `worker` warten, bis die Migrationen abgeschlossen sind.

   > **Hinweis für Profile-basierte Deployments (Demo/Staging):** Wenn deine `docker-compose.yml` Docker Compose Profiles verwendet (`profiles: [demo]`, `profiles: [staging]`), muss das `--profile`-Flag bei jedem Befehl angegeben werden:
   > ```bash
   > docker compose --profile demo pull
   > docker compose --profile demo up -d
   > ```
   > Ohne `--profile` werden nur die Default-Services (z. B. Caddy) gestartet — die App-Container werden ignoriert.

4. **Migrationen prüfen:**
   ```bash
   docker compose logs -f migrate
   ```
   Der Container sollte mit Exit-Code 0 beendet sein und alle Migrationen als erfolgreich melden.

5. **Alle Container prüfen:**
   ```bash
   docker compose ps
   ```
   Alle Container sollten den Status `healthy` bzw. `running` haben.

---

### Neue Feature-Entwicklung (als Entwickler)

1. Feature entwickeln, committen und pushen:
   ```bash
   git commit -m "feat: ..."
   git push origin main
   ```

2. CI baut neue Docker-Images und veröffentlicht sie auf GHCR.

3. Auf dem Live-Server übernimmt **Watchtower automatisch** — oder manuell:
   ```bash
   docker compose pull && docker compose up -d
   ```

4. `migrate` läuft automatisch beim Neustart — keine manuelle Migration nötig.

---

### Sonderfall: Erstmaliger Einsatz der neuen migrate-Service-Konfiguration

Wer noch eine ältere `docker-compose.yml` **ohne** den `migrate`-Service hat, führt einmalig folgendes aus:

```bash
docker compose run --rm migrate   # Migrationen einmalig manuell anstoßen
docker compose up -d              # Danach normal starten
```

Ab diesem Zeitpunkt läuft `migrate` bei jedem `docker compose up -d` automatisch.

---

## 8. Admin-CLI

Der API-Container enthält ein Admin-Werkzeug für Wartungsaufgaben.

```bash
# Health-Check
docker compose exec api /admin health-check

# Alle Organisationen auflisten
docker compose exec api /admin list-orgs

# Benutzer einer Organisation auflisten
docker compose exec api /admin list-users --org meine-firma

# Passwort zurücksetzen
docker compose exec api /admin reset-password user@example.com neues-passwort
```

Mit lokaler Go-Installation alternativ:

```bash
cd backend && go run ./cmd/admin --help
```

---

## 9. KI-Compliance-Berater konfigurieren

Vakt enthält einen KI-Berater, der auf Basis der echten Compliance-Lücken priorisierte Handlungsempfehlungen generiert ("Was soll ich diese Woche tun?"). Er läuft **standardmäßig** lokal auf der CPU — kein GPU, kein Cloud-Account nötig.

### Standard: Ollama lokal (kein GPU, kein API-Key)

Ollama startet automatisch mit `docker compose up`. Das Default-Modell wird beim ersten Start einmalig vom `ollama-init`-Container gezogen (~4.5 GB) — kein manueller Schritt nötig.

Zum Wechseln des Modells:

```bash
docker compose exec ollama ollama pull qwen2.5:3b
```

Empfohlene CPU-taugliche Modelle (kein VRAM nötig):

| Modell | Stärke |
|---|---|
| `qwen2.5:7b` | Standard — beste DE-Compliance-Qualität (~4.5 GB) |
| `qwen2.5:3b` | Leichter — für Server mit < 16 GB RAM (~1.9 GB) |
| `phi3.5:mini` | Sehr schnell, gutes Reasoning (~2.3 GB) |
| `llama3.2:3b` | Gutes Deutsch, schnell (~2.0 GB) |

Das Modell wird einmalig in das Volume `ollama_models` geladen und bleibt über Updates hinweg erhalten.

### KI deaktivieren

```env
VAKT_AI_PROVIDER=disabled
```

### Alternative: Cloud-Provider (EU, DSGVO-freundlich)

[Mistral AI](https://mistral.ai) (Paris) — ca. **€0,001 pro Anfrage**, keine US-Datenweitergabe:

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

Funktioniert auch mit OpenAI, Groq, LM Studio oder jedem anderen OpenAI-kompatiblen Anbieter.

---

## 10. Pro-Lizenz aktivieren

Vakt Community ist kostenlos und für immer nutzbar. **Pro** schaltet spezialisierte Frameworks, erweiterte Module und Integrationen frei — Kauf über [vakt.norvikops.de](https://vakt.norvikops.de/#preise) (Polar-Checkout, 30 Tage kostenlose Testphase).

Nach dem Kauf bekommst du deinen License Key per E-Mail. Es gibt **zwei Wege**, ihn zu aktivieren:

### Primär: manuell (offline, keine ausgehende Verbindung)

Der empfohlene Weg für ein self-hosted Produkt — nichts verlässt deine Infrastruktur.

1. License Key aus der Kauf-E-Mail kopieren.
2. In Vakt einloggen (Admin), **Einstellungen → Lizenz** öffnen.
3. Key einfügen → **Aktivieren**.

Bei jeder Verlängerung (und bei Umwandlung der Testphase in ein bezahltes Abo) kommt automatisch ein neuer Key per Mail — einmal einfügen, fertig. Ab 30 Tage vor Ablauf erinnert ein Banner in der Oberfläche.

### Optional: automatische Erneuerung (Opt-in, ausgehende Verbindung)

Wer den manuellen Schritt sparen will, setzt in der `.env`:

```env
VAKT_LICENSE_TOKEN=<Renewal-Token aus der Kauf-E-Mail>
```

Die Instanz holt sich dann täglich den aktuellen Key von `api.norvikops.de` — kein Handanlegen bei Verlängerungen. **Das ist eine bewusste Opt-in-Ausnahme vom No-Phone-Home-Prinzip:** es wird ausschließlich der Token übertragen, keine Geschäfts- oder Nutzungsdaten. In Umgebungen mit strikter Egress-Kontrolle (ISO 27001 / NIS2) entweder `api.norvikops.de:443` in die Whitelist eintragen — oder den Token weglassen und beim manuellen Weg bleiben. Details: [Konfiguration → VAKT_LICENSE_TOKEN](wiki/configuration.md).

---

## 11. Monitoring

### Health-Endpunkte

| Endpunkt | Beschreibung | Verwendung |
|---|---|---|
| `GET /health` | Liveness-Check | Kubernetes liveness probe |
| `GET /health/ready` | Readiness-Check (DB + Redis) | Kubernetes readiness probe, Load Balancer |

### Prometheus-Metriken

```
http://localhost:8080/metrics
```

Für Produktionsmonitoring empfehlen wir **Grafana + Prometheus**. Eine fertige `docker-compose.monitoring.yml` mit vorkonfigurierten Dashboards ist in Planung.

---

## 12. Kubernetes (Helm)

```bash
helm install vakt ./helm/vakt \
  --set secret.key=$(openssl rand -hex 32) \
  --set database.url=postgres://vakt:pass@postgres:5432/vakt?sslmode=disable \
  --set redis.url=redis://redis:6379
```

Alle verfügbaren Helm-Werte sind in `helm/vakt/values.yaml` dokumentiert.

---

## 13. Troubleshooting

### Container-Logs anzeigen

```bash
docker compose logs -f api       # API-Logs
docker compose logs -f worker    # Worker-Logs
docker compose logs -f postgres  # Datenbank-Logs
```

### Häufige Probleme

**Datenbank nicht erreichbar beim Start**

```bash
# Healthcheck-Status aller Container prüfen
docker compose ps

# PostgreSQL-Logs anzeigen
docker compose logs postgres
```

Der API-Container startet erst, wenn PostgreSQL als `healthy` gemeldet ist. Das dauert normalerweise 5–15 Sekunden.

**Migrationen fehlgeschlagen**

```bash
docker compose logs migrate
```

Häufige Ursache: `VAKT_DB_URL` ist falsch konfiguriert oder der Datenbankbenutzer hat keine ausreichenden Rechte.

**DB unavailable — all routes disabled**

```
{"level":"warn","message":"DB unavailable — all routes disabled"}
```

Die API deaktiviert alle Routes wenn sie die Datenbank nicht erreicht. Ursache ist fast immer ein Passwort-Mismatch zwischen `.env` und der PostgreSQL-Volume.

Diagnose:

```bash
docker compose logs api | head -5
```

Fix — DB-Passwort aktualisieren:

```bash
# 1. Starkes Passwort in .env setzen
nano .env   # POSTGRES_PASSWORD=<neues_passwort>

# 2. DB-User-Passwort anpassen
docker compose exec postgres psql -U vakt -c \
  "ALTER USER vakt WITH PASSWORD '<neues_passwort>';"

# 3. API neu starten
docker compose up -d api
```

> **Wichtig:** Das Passwort in der PostgreSQL-Volume wird beim ersten Start des DB-Containers gesetzt und danach **nicht** automatisch geändert, selbst wenn `.env` aktualisiert wird. Nach einer Volume-Neuerstellung oder einem Passwort-Wechsel muss der `ALTER USER`-Befehl manuell ausgeführt werden.

**migrate-Container schlägt fehl**

```bash
# Logs des migrate-Containers prüfen
docker compose logs migrate

# Sicherstellen, dass PostgreSQL healthy ist
docker compose ps postgres
```

Wenn PostgreSQL noch nicht bereit ist, einfach erneut versuchen:

```bash
docker compose run --rm migrate
```

Starte erst danach `api` und `worker` mit `docker compose up -d`.

**Port bereits belegt**

```bash
ss -tlnp | grep :80
ss -tlnp | grep :443
```

Den blockierenden Prozess beenden oder Vakt auf einem anderen Port starten (Port-Zuordnung des `caddy`-Service in `docker-compose.yml` anpassen).

**Secret Key fehlt oder ist der Standard-Wert**

```bash
grep VAKT_SECRET_KEY .env
```

Der Wert darf nicht der Platzhalter aus `.env.example` (`ERSETZEN_SIE_DIESEN_WERT`) sein. Neuen Key generieren:

```bash
openssl rand -hex 32
```

> Den generierten Key sicher aufbewahren. Wird er nach dem ersten Start geändert, sind alle verschlüsselten Daten in der Datenbank nicht mehr lesbar.
