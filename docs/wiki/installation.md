# Installation

Diese Anleitung führt dich von null bis zu einer laufenden Vakt-Instanz.

---

## Systemanforderungen

| | Minimum | Empfohlen | Mit KI-Berater (Standard) |
|---|---|---|---|
| **CPU** | 2 vCPU | 4 vCPU | 4 vCPU — kein GPU nötig |
| **RAM** | 2 GB | 4 GB | 4 GB (+2 GB für das Modell) |
| **Disk** | 20 GB SSD | 40 GB SSD | 40 GB SSD (+3 GB für das Modell) |
| **Docker Engine** | 24+ | 24+ | 24+ |
| **Docker Compose** | v2 | v2 | v2 |
| **Betriebssystem** | Linux (empfohlen), macOS, Windows (WSL2) | Linux | Linux |

Der KI-Berater läuft standardmäßig lokal über einen Ollama-Container — rein auf CPU, kein GPU, kein Cloud-API-Key erforderlich. Wer ihn nicht braucht, kann ihn mit `VAKT_AI_PROVIDER=disabled` abschalten.

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
| `VAKT_AI_MODEL` | `llama3.2:3b` | Modellname |

### SMTP (für Vakt Aware und Benachrichtigungen)

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `VAKT_SMTP_HOST` | `localhost` | SMTP-Server |
| `VAKT_SMTP_PORT` | `1025` | SMTP-Port |
| `VAKT_SMTP_USER` | — | Benutzername (erforderlich für Port 587/465) |
| `VAKT_SMTP_PASS` | — | Passwort (erforderlich für Port 587/465) |
| `VAKT_SMTP_FROM` | `secreflex@vakt.local` | Absenderadresse |

---

## KI-Berater einrichten

### Option A: Lokal mit Ollama (Standard, empfohlen)

Der Ollama-Container ist bereits in `docker-compose.yml` enthalten. Nach dem ersten Start das Modell einmalig laden:

```bash
docker compose exec ollama ollama pull llama3.2:3b
```

Das Modell (~2 GB) wird lokal gespeichert und steht danach offline zur Verfügung.

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

---

## HTTPS mit eigenem Zertifikat

Vakt liefert eine Nginx-Konfiguration mit. Für HTTPS gibt es zwei Varianten.

### Variante A: Let's Encrypt (öffentlich erreichbarer Server)

In `docker-compose.yml` den Certbot-Service aktivieren (oder Certbot auf dem Host laufen lassen) und in `nginx/nginx.conf` den HTTPS-Block einkommentieren:

```nginx
server {
    listen 443 ssl;
    server_name deine-domain.de;

    ssl_certificate     /etc/letsencrypt/live/deine-domain.de/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/deine-domain.de/privkey.pem;

    location / {
        proxy_pass http://frontend:5173;
    }

    location /api/ {
        proxy_pass http://api:8080;
    }
}
```

### Variante B: Eigenes Zertifikat (internes Netz / Self-signed)

Zertifikat und Key in `nginx/certs/` ablegen und in `nginx.conf` referenzieren:

```nginx
ssl_certificate     /etc/nginx/certs/cert.pem;
ssl_certificate_key /etc/nginx/certs/key.pem;
```

Anschließend `VAKT_FRONTEND_URL` auf die HTTPS-URL setzen:

```env
VAKT_FRONTEND_URL=https://vakt.intranet.meine-firma.de
```

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
6. **KI-Modell laden** — Falls Ollama genutzt wird: `docker compose exec ollama ollama pull llama3.2:3b`.
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
