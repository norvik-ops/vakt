# Vakt — Konfigurationsreferenz

Alle Konfigurationswerte werden über Umgebungsvariablen gesetzt. In Docker-Deployments wird die Datei `.env` im Projektverzeichnis verwendet (`env_file: .env` in `docker-compose.yml`).

Eine vollständige Vorlage aller Variablen findest du in `.env.example`.

---

## Datenbank

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_DB_URL` | ✅ | – | PostgreSQL-Verbindungsstring. Format: `postgres://user:pass@host:5432/db?sslmode=disable` |
| `POSTGRES_PASSWORD` | – | `vakt` | Passwort für den PostgreSQL-Container (wird von `docker-compose.yml` ausgelesen). Muss mit dem Passwort in `VAKT_DB_URL` übereinstimmen. |

**Beispiel:**

```env
VAKT_DB_URL=postgres://vakt:vakt@postgres:5432/vakt?sslmode=disable
POSTGRES_PASSWORD=vakt
```

---

## Redis

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_REDIS_URL` | ✅ | – | Redis-Verbindungsstring. Format: `redis://host:6379` oder `redis://:passwort@host:6379` |

**Beispiel:**

```env
VAKT_REDIS_URL=redis://redis:6379
```

---

## Sicherheit

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_SECRET_KEY` | ✅ | – | 32-Byte Hex-Master-Key für AES-256-GCM-Verschlüsselung aller Secrets in der Datenbank. Generieren: `openssl rand -hex 32`. **Nie nach dem ersten Start ändern** (siehe Hinweis unten). |

**Beispiel:**

```env
VAKT_SECRET_KEY=$(openssl rand -hex 32)   # Beispiel — echten Wert generieren!
```

---

## Anwendung

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `APP_VERSION` | – | `0.1.0` | Versionsnummer der Anwendung. Wird im `/health`-Endpunkt zurückgegeben. |
| `VAKT_API_PORT` | – | `8080` | Port, auf dem der API-Server lauscht (innerhalb des Containers). |
| `VAKT_MODULES_ENABLED` | – | alle aktiv | Kommaseparierte Liste der aktivierten Module. Mögliche Werte: `secpulse`, `secvitals`, `secvault`, `secreflex`, `secprivacy`. |
| `AUTO_MIGRATE` | – | `false` | Wenn `true`, führt der API-Container beim Start automatisch ausstehende Datenbankmigrationen aus. |
| `VAKT_DEMO` | – | `false` | Wenn `true`, werden Beispieldaten eingespielt und der ephemere Demo-Flow aktiviert: jeder Login-Besucher bekommt eine eigene Demo-Org mit Random-Credentials, die für 4 Stunden gilt (siehe [Demo-Modus](wiki/demo-mode.md)). |
| `VAKT_FRONTEND_URL` | – | `http://localhost:5173` | Öffentlich erreichbare URL des Frontends. Wird von Vakt Aware für Tracking-Pixel und Klick-Links in Kampagnen-E-Mails verwendet. In Produktion auf die echte Domain setzen. |

**Beispiel:**

```env
APP_VERSION=0.1.0
VAKT_API_PORT=8080
VAKT_MODULES_ENABLED=secpulse,secvitals,secvault,secreflex,secprivacy
AUTO_MIGRATE=false
VAKT_DEMO=false
VAKT_FRONTEND_URL=https://vakt.meine-firma.de
```

---

## Update-Benachrichtigungen

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_UPDATE_CHECK` | – | `false` | Aktiviert den täglichen Check auf neue Vakt-Versionen via GitHub Releases API. Zeigt Admins ein Banner in der UI wenn eine neue Version verfügbar ist. Kein Datenaustausch — nur lesender Zugriff auf die öffentliche GitHub-API. |

---

## SMTP (Vakt Aware & Scheduled Reports)

Vakt Aware benötigt einen SMTP-Server, um Phishing-Simulations-E-Mails zu versenden. Auch **Scheduled Reports** (automatisch geplante Compliance-, Findings- und Risk-Berichte) verwenden dieselbe SMTP-Konfiguration. Für Entwicklung und Tests ist [Mailpit](https://github.com/axllent/mailpit) vorkonfiguriert (Port 1025, keine Authentifizierung).

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_SMTP_HOST` | – | `localhost` | Hostname des SMTP-Servers. |
| `VAKT_SMTP_PORT` | – | `1025` | SMTP-Port. `1025` für Mailpit (Entwicklung), `587` für STARTTLS (Produktion), `465` für SSL/TLS. |
| `VAKT_SMTP_USER` | – | – | SMTP-Benutzername. Erforderlich für Port 587/465 (Produktions-SMTP). |
| `VAKT_SMTP_PASS` | – | – | SMTP-Passwort. Erforderlich für Port 587/465 (Produktions-SMTP). |
| `VAKT_SMTP_FROM` | – | `secreflex@example.com` | Absenderadresse für alle Kampagnen-E-Mails. Muss eine gültige Adresse sein, die der SMTP-Server akzeptiert. |

**Beispiel Entwicklung (Mailpit):**

```env
VAKT_SMTP_HOST=localhost
VAKT_SMTP_PORT=1025
VAKT_SMTP_FROM=secreflex@example.com
```

**Beispiel Produktion:**

```env
VAKT_SMTP_HOST=smtp.mein-anbieter.de
VAKT_SMTP_PORT=587
VAKT_SMTP_USER=secreflex@meine-firma.de
VAKT_SMTP_PASS=sicheres-passwort
VAKT_SMTP_FROM=secreflex@meine-firma.de
```

---

## AI-Berichte (optional)

Vakt kann automatisch Compliance-Berichte über einen OpenAI-kompatiblen Provider generieren. Standardmäßig deaktiviert. Unterstützt werden OpenAI, Mistral AI, Groq, Ollama, LM Studio und jeder weitere OpenAI-kompatible Endpunkt.

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `VAKT_AI_PROVIDER` | – | `disabled` | AI-Provider aktivieren. Aktuell unterstützte Werte: `disabled`, `openai` (für alle OpenAI-kompatiblen Endpunkte). |
| `VAKT_AI_BASE_URL` | – | – | API-Basisendpunkt des Providers. Beispiele: `https://api.mistral.ai/v1`, `https://api.openai.com/v1`, `http://ollama:11434`. |
| `VAKT_AI_API_KEY` | – | – | API-Key des Providers. Für lokale Provider wie Ollama oder LM Studio leer lassen. |
| `VAKT_AI_MODEL` | – | `mistral-small-latest` | Modellname, der für Berichtsgenerierung verwendet wird. |

**Beispiel Mistral AI (empfohlen — EU-Server, DSGVO-freundlich, ca. €0,001 pro Bericht):**

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

**Beispiel Ollama (lokal, kein API-Key erforderlich):**

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=http://ollama:11434
VAKT_AI_MODEL=llama3.2
```

---

## Benutzerverwaltung

Vakt unterstützt vier Rollen in der Community Edition:

| Rolle | Rechte |
|-------|--------|
| **Admin** | Vollzugriff — Benutzer verwalten, Module konfigurieren |
| **Analyst** | Lesen + Schreiben in allen Modulen |
| **Viewer** | Nur lesen — alle Module |
| **Auditor** | Nur lesen + Audit-Bericht exportieren |

Mit einer **Pro**-Lizenz werden zusätzlich granulare Modul-Berechtigungen freigeschaltet: Pro Benutzer kann `can_read` und `can_write` je Modul separat gesetzt werden. Verwaltung über **Einstellungen → Benutzerverwaltung**.

---

## Externe Authentifizierung — OIDC/SAML SSO (optional)

Vakt unterstützt Single Sign-On über [Casdoor](https://casdoor.org) als OIDC/SAML-Proxy. Damit können bestehende Identity-Provider (Azure AD, Okta, Keycloak, Google Workspace) eingebunden werden.

| Variable | Pflicht | Standard | Beschreibung |
|---|---|---|---|
| `CASDOOR_URL` | – | – | URL des Casdoor-Servers. Beispiel: `https://auth.meine-firma.de` |
| `CASDOOR_CLIENT_ID` | – | – | OAuth2 / OIDC Client-ID der Vakt-Anwendung in Casdoor. |
| `CASDOOR_CLIENT_SECRET` | – | – | OAuth2 / OIDC Client-Secret. Sicher aufbewahren, nicht in Git committen. |

Casdoor-Einrichtung: Siehe [Casdoor-Dokumentation](https://casdoor.org/docs/overview) und die Vakt-Architektur-Dokumentation (`docs/architecture.md`).

---

## Wichtige Hinweise

### `VAKT_SECRET_KEY` nie ändern

Der Master-Key wird zur AES-256-GCM-Verschlüsselung aller Secrets in der Datenbank (Vakt Vault-Einträge, SMTP-Passwörter, API-Keys) verwendet. Wird der Key nach dem ersten Deployment geändert, sind alle verschlüsselten Daten **dauerhaft unlesbar**.

- Key vor dem ersten Start generieren: `openssl rand -hex 32`
- Key sicher speichern (Passwortmanager, Vault)
- Key **niemals** in Git committen

### `AUTO_MIGRATE` nur kontrolliert einsetzen

`AUTO_MIGRATE=true` ist praktisch für einfache Setups und beim ersten Start. In Produktionsumgebungen mit kritischen Daten empfehlen wir:

1. Backup anlegen
2. Migration manuell mit `docker compose exec api /api migrate` ausführen
3. Ergebnis prüfen
4. Erst dann den neuen Anwendungscode starten

### `VAKT_DEMO=true` nur für Test-Umgebungen

Demo-Modus enthält Beispieldaten und einen öffentlich erreichbaren `/api/v1/demo/start`-Endpoint. Diese Option **niemals** in Produktionsumgebungen mit echten Compliance-Daten aktivieren. Nach einer Demo-Installation für den produktiven Einsatz:

1. `VAKT_DEMO=false` setzen
2. Ephemerale Demo-Orgs (Slug-Muster `demo-*`) löschen
3. Anwendung neu starten
