# Installation

Diese Anleitung führt dich von null bis zu einer laufenden Vakt-Instanz.

---

## Systemanforderungen

| | Minimum | Empfohlen | Mit KI (Minimum) | Mit KI (Empfohlen) |
|---|---|---|---|---|
| **CPU** | 2 vCPU | 4 vCPU | 4 vCPU | 4 vCPU |
| **RAM** | 2 GB | 4 GB | **8 GB** | **16 GB** |
| **Disk** | 20 GB SSD | 40 GB SSD | 40 GB SSD (+5 GB für das Modell) | 40 GB SSD (+5 GB für das Modell) |
| **Docker Engine** | 24+ | 24+ | 24+ | 24+ |
| **Docker Compose** | v2 | v2 | v2 | v2 |
| **Betriebssystem** | Linux (empfohlen), macOS, Windows (WSL2) | Linux | Linux | Linux |

Der KI-Berater läuft standardmäßig lokal über einen Ollama-Container — rein auf CPU, kein GPU, kein Cloud-API-Key erforderlich. Das Default-Modell `qwen2.5:7b` (~4.5 GB RAM, Apache 2.0) wird beim ersten `docker compose up` automatisch gezogen; kein manueller Schritt nötig. Wer ihn nicht braucht, kann ihn mit `VAKT_AI_PROVIDER=disabled` abschalten — zusätzlich sollten `ollama` und `ollama-init` gestoppt werden (`docker compose stop ollama ollama-init`), damit der RAM-Footprint entfällt.

---

## Docker Compose Quickstart

### 1. Repository klonen

```bash
git clone https://github.com/norvik-ops/vatk
cd vatk
```

### 2. Konfiguration vorbereiten

```bash
cp .env.example .env
```

Dann den Master-Key setzen — einmalig, **nicht mehr ändern nach dem ersten Start**:

```bash
sed -i 's/VAKT_SECRET_KEY=.*/VAKT_SECRET_KEY='"$(openssl rand -hex 32)"'/' .env
```

Oder manuell in `.env` eintragen:

```env
VAKT_SECRET_KEY=<Ausgabe von: openssl rand -hex 32>
```

### 3. Starten

```bash
docker compose up -d
```

Vakt startet und ist nach ca. 30–60 Sekunden unter `http://localhost` erreichbar. Datenbankmigrationen laufen automatisch beim ersten Start.

### 4. Ersten Benutzer anlegen

Beim ersten Aufruf erscheint der Setup-Wizard. Dort legst du die Organisation und den ersten Admin-Account an. Danach ist der Setup-Wizard dauerhaft deaktiviert.

---

## Umgebungsvariablen — Überblick

Die vollständige Referenz aller Variablen findest du in der [Konfigurationsreferenz](configuration.md). Für den Start sind folgende Variablen relevant:

### Pflichtfelder

| Variable | Beschreibung |
|----------|--------------|
| `VAKT_DB_URL` | PostgreSQL-Verbindungsstring |
| `VAKT_REDIS_URL` | Redis-Verbindungsstring |
| `VAKT_SECRET_KEY` | 32-Byte Hex-Master-Key (AES-256-GCM) — niemals nach erstem Start ändern |

### Wichtige optionale Felder

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `VAKT_MODULES_ENABLED` | alle | Kommagetrennte Liste aktiver Module |
| `AUTO_MIGRATE` | `false` | Migrationen automatisch beim Start ausführen |
| `VAKT_FRONTEND_URL` | `http://localhost:5173` | Öffentliche URL des Frontends (für E-Mail-Links) |
| `VAKT_AI_PROVIDER` | `openai` | KI-Provider (`openai` oder `disabled`) |
| `VAKT_AI_BASE_URL` | `http://ollama:11434/v1` | API-Endpunkt des KI-Providers |
| `VAKT_AI_MODEL` | `qwen2.5:7b` | Modellname (Default; Apache 2.0; ~4.5 GB RAM, braucht 8 GB; auf kleinen VMs `qwen2.5:3b`) |

### SMTP (für Vakt Aware und Benachrichtigungen)

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `VAKT_SMTP_HOST` | `localhost` | SMTP-Server |
| `VAKT_SMTP_PORT` | `1025` | SMTP-Port |
| `VAKT_SMTP_USER` | — | Benutzername (erforderlich für Port 587/465) |
| `VAKT_SMTP_PASS` | — | Passwort (erforderlich für Port 587/465) |
| `VAKT_SMTP_FROM` | `vaktaware@vakt.local` | Absenderadresse |

---

## KI-Berater einrichten

### Option A: Lokal mit Ollama (Standard, empfohlen)

Der Ollama-Container ist bereits in `docker-compose.yml` enthalten und startet automatisch mit `docker compose up`. Der `ollama-init`-Container zieht das Default-Modell `qwen2.5:7b` beim ersten Start automatisch (~4.5 GB, einmalig). Kein manueller Schritt nötig — nach dem Download steht die KI offline zur Verfügung.

Modell wechseln (optional):

```bash
# Anderes Modell ziehen
docker compose exec ollama ollama pull phi3.5:mini

# In .env anpassen
VAKT_AI_MODEL=phi3.5:mini

# API neu starten
docker compose restart api worker
```

### Option B: Cloud-Provider (z. B. Mistral AI)

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

Mistral AI nutzt EU-Server und ist DSGVO-freundlich. Kosten: ca. €0,001 pro Bericht.

Alle OpenAI-kompatiblen Endpunkte funktionieren: OpenAI, Mistral, Groq, Ollama, LM Studio, vLLM.

### KI deaktivieren

```env
VAKT_AI_PROVIDER=disabled
```

Damit werden die KI-Buttons im UI ausgeblendet. Um auch den RAM-Footprint zu entfernen, Ollama-Container stoppen:

```bash
docker compose stop ollama ollama-init
```

Alternativ die Services `ollama` und `ollama-init` aus der `docker-compose.yml` entfernen (empfohlen für dauerhaft kleine VMs mit < 8 GB RAM).

---

## HTTPS

Vakt läuft standardmäßig auf HTTP (Port 80). Für die meisten internen Installationen ist das ausreichend — die VM ist typischerweise nicht direkt aus dem Internet erreichbar, und TLS wird durch einen vorgelagerten Load-Balancer oder Reverse-Proxy der eigenen Infrastruktur terminiert.

### Variante A: Eigenes Zertifikat (empfohlen für interne Installationen)

Das Repository enthält ein HTTPS-Overlay und ein Skript zur Zertifikatserstellung:

```bash
# Zertifikat erzeugen (nutzt mkcert wenn vorhanden, sonst openssl)
./scripts/gen-local-cert.sh

# Stack mit HTTPS starten
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d
```

Das Skript legt `nginx/certs/localhost.crt` und `nginx/certs/localhost.key` an. Mit einem Zertifikat des eigenen internen CA einfach die Dateien direkt dort ablegen — das Skript überschreibt nichts, wenn die Dateien bereits existieren.

Anschließend `VAKT_FRONTEND_URL` auf die HTTPS-URL setzen:

```env
VAKT_FRONTEND_URL=https://vakt.intranet.meine-firma.de
```

### Variante B: Caddy als Reverse-Proxy (für öffentlich erreichbare Server)

Wenn Vakt auf einem Server mit öffentlichem DNS läuft, übernimmt [Caddy](https://caddyserver.com) TLS-Zertifikate vollautomatisch via Let's Encrypt — kein Certbot, keine Cronjobs, keine manuelle Erneuerung.

```bash
# Caddy auf dem Host installieren (Ubuntu/Debian)
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | \
    sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | \
    sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy
```

`/etc/caddy/Caddyfile`:

```caddyfile
deine-domain.de {
    reverse_proxy localhost:80
}
```

Der Docker-Stack bleibt unverändert auf Port 80. Caddy übernimmt Port 80/443, holt das Zertifikat automatisch und erneuert es ohne Eingriff.

---

## LDAP / Active Directory

Vakt kann Benutzerkonten aus einem LDAP/AD synchronisieren:

```env
VAKT_LDAP_URL=ldap://dc.meine-firma.local:389
VAKT_LDAP_BIND_DN=CN=vakt-service,OU=ServiceAccounts,DC=meine-firma,DC=local
VAKT_LDAP_BIND_PASS=geheimes-passwort
VAKT_LDAP_BASE_DN=OU=Users,DC=meine-firma,DC=local
VAKT_LDAP_USER_FILTER=(objectClass=person)
VAKT_LDAP_GROUP_FILTER=(objectClass=group)
VAKT_LDAP_TLS=false
```

---

## OIDC / SAML Single Sign-On

SSO wird über [Casdoor](https://casdoor.org) als Proxy unterstützt. Damit lassen sich Azure AD, Okta, Keycloak und Google Workspace einbinden:

```env
CASDOOR_URL=https://auth.meine-firma.de
CASDOOR_CLIENT_ID=vakt-app
CASDOOR_CLIENT_SECRET=geheimes-secret
```

---

## Updates

```bash
git pull
docker compose pull
docker compose up -d
```

### Update-Benachrichtigungen (opt-in)

Vakt prüft nicht automatisch auf neue Versionen. Wenn du informiert werden möchtest, wenn eine neue Version verfügbar ist, gibt es zwei Möglichkeiten:

**Option 1 — In-App-Banner:** Setze `VAKT_UPDATE_CHECK=true` in deiner `.env`. Vakt prüft dann einmal täglich die [GitHub Releases API](https://github.com/norvik-ops/vatk/releases) und zeigt Administratoren einen Hinweis-Banner in der Oberfläche. Es werden dabei keine Daten gesendet.

**Option 2 — Watchtower:** Für automatische Container-Updates siehe die [Deployment-Dokumentation](../setup.md).

Migrationen laufen automatisch beim Start (wenn `AUTO_MIGRATE=true` gesetzt ist). Bei kritischen Produktionssystemen empfiehlt sich der manuelle Ablauf:

```bash
# 1. Backup anlegen
# 2. Migration testen
docker compose exec api /api migrate
# 3. Anwendung starten
docker compose up -d
```

---

## Kubernetes (Helm)

Ein Helm Chart liegt unter `helm/vakt/`. Grundlegender Aufruf:

```bash
helm install vakt ./helm/vakt \
  --set secret.key=$(openssl rand -hex 32) \
  --set postgresql.postgresqlPassword=sicher \
  --set ingress.enabled=true \
  --set ingress.hostname=vakt.meine-firma.de
```

---

## Erste Schritte nach dem Setup

1. **Organisation konfigurieren** — Sector und Bundesland setzen (wichtig für automatische Behördenauswahl in Vakt Comply).
2. **Frameworks aktivieren** — In Vakt Comply die relevanten Standards aktivieren (NIS2, ISO 27001, BSI-Grundschutz o. a.).
3. **Benutzer einladen** — Über Einstellungen → Benutzerverwaltung weitere Teammitglieder einladen.
4. **2FA aktivieren** — Für Admin-Accounts TOTP einrichten (Einstellungen → Sicherheit).
5. **SMTP konfigurieren** — Für Benachrichtigungen und Vakt-Aware-Kampagnen einen SMTP-Server eintragen.
6. **KI-Modell laden** — Falls Ollama genutzt wird: `docker compose exec ollama ollama pull qwen2.5:7b`.
7. **Ersten Scan starten** — In Vakt Scan ein Asset anlegen und einen Trivy-Scan auslösen.

---

## Gesundheitsstatus prüfen

```bash
# Liveness
curl http://localhost/health

# Readiness (prüft DB und Redis)
curl http://localhost/health/ready
```

Beide Endpunkte antworten mit HTTP 200 wenn alles läuft.

---

## Datensicherung

Vakt speichert alle Daten in PostgreSQL. Eine einfache Backup-Strategie:

```bash
docker compose exec postgres pg_dump -U vakt vakt > backup-$(date +%Y%m%d).sql
```

Hochgeladene Dateien (Evidence-Anhänge) liegen im Volume `./data/uploads` und müssen separat gesichert werden.
