# Konfigurationsreferenz

Alle Konfigurationswerte werden über Umgebungsvariablen gesetzt. In Docker-Deployments wird die Datei `.env` im Projektverzeichnis verwendet (`env_file: .env` in `docker-compose.yml`). Eine Vorlage aller Variablen mit Kommentaren liegt in `.env.example`.

---

## Datenbank

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_DB_URL` | Ja | — | PostgreSQL-Verbindungsstring. Format: `postgres://user:pass@host:5432/db?sslmode=disable` |
| `POSTGRES_PASSWORD` | — | `vakt` | Passwort für den PostgreSQL-Container (wird von `docker-compose.yml` ausgelesen). Muss mit dem Passwort in `VAKT_DB_URL` übereinstimmen. |

**Beispiel:**

```env
VAKT_DB_URL=postgres://vakt:sicherespasswort@postgres:5432/vakt?sslmode=disable
POSTGRES_PASSWORD=sicherespasswort
```

---

## Redis

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_REDIS_URL` | Ja | — | Redis-Verbindungsstring. Format: `redis://host:6379` oder `redis://:passwort@host:6379` |

**Beispiel:**

```env
VAKT_REDIS_URL=redis://redis:6379
```

---

## Sicherheit

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_SECRET_KEY` | Ja | — | 32-Byte Hex-Master-Key für AES-256-GCM-Verschlüsselung aller Secrets in der Datenbank. Generieren: `openssl rand -hex 32`. **Nie nach dem ersten Start ändern.** |

**Beispiel:**

```env
VAKT_SECRET_KEY=$(openssl rand -hex 32)   # Beispiel — echten Wert generieren!
```

> **Wichtig:** Der Master-Key wird zur AES-256-GCM-Verschlüsselung aller Secrets (Vakt-Vault-Einträge, SMTP-Passwörter, API-Keys) verwendet. Wird der Key nach dem ersten Deployment geändert, sind alle verschlüsselten Daten dauerhaft unlesbar. Key sicher in einem Passwortmanager oder Vault aufbewahren, niemals in Git committen.

---

## Anwendung

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_API_PORT` | — | `8080` | Port, auf dem der API-Server innerhalb des Containers lauscht. |
| `APP_VERSION` | — | `0.1.0` | Versionsnummer. Wird im `/health`-Endpunkt zurückgegeben. |
| `VAKT_MODULES_ENABLED` | — | alle | Kommagetrennte Liste aktiver Module. Mögliche Werte: `vaktscan`, `vaktcomply`, `vaktvault`, `vaktaware`, `vaktprivacy`, `sechr`. |
| `AUTO_MIGRATE` | — | `false` | Wenn `true`, führt der API-Container beim Start automatisch ausstehende Datenbankmigrationen aus. |
| `VAKT_FRONTEND_URL` | — | `http://localhost:5173` | Öffentlich erreichbare URL des Frontends. Wird für E-Mail-Links in Benachrichtigungen, Vakt-Aware-Kampagnen und Policy-Akzeptanz-E-Mails verwendet. In Produktion auf die echte Domain setzen. |
| `VAKT_UPLOAD_DIR` | — | `./data/uploads` | Verzeichnis für hochgeladene Dateien (Evidence-Anhänge). In Docker-Deployments als Volume mounten. |

**Beispiel:**

```env
VAKT_API_PORT=8080
APP_VERSION=1.0.0
VAKT_MODULES_ENABLED=vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,sechr
AUTO_MIGRATE=false
VAKT_FRONTEND_URL=https://vakt.meine-firma.de
VAKT_UPLOAD_DIR=/data/uploads
```

---

## SMTP (Benachrichtigungen und Vakt Aware)

SMTP wird für Benachrichtigungs-E-Mails, Phishing-Simulations-Kampagnen (Vakt Aware) und Policy-Akzeptanz-Links (Vakt Comply) benötigt.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_SMTP_HOST` | — | `localhost` | Hostname des SMTP-Servers. |
| `VAKT_SMTP_PORT` | — | `1025` | SMTP-Port. `1025` für Mailpit (Entwicklung), `587` für STARTTLS, `465` für SSL/TLS. |
| `VAKT_SMTP_USER` | — | — | SMTP-Benutzername. Erforderlich für Port 587/465. |
| `VAKT_SMTP_PASS` | — | — | SMTP-Passwort. Erforderlich für Port 587/465. |
| `VAKT_SMTP_FROM` | — | `vaktaware@vakt.local` | Absenderadresse für alle E-Mails. Muss eine gültige Adresse sein, die der SMTP-Server akzeptiert. |

**Beispiel Entwicklung (Mailpit):**

```env
VAKT_SMTP_HOST=localhost
VAKT_SMTP_PORT=1025
VAKT_SMTP_FROM=vakt@beispiel.de
```

**Beispiel Produktion:**

```env
VAKT_SMTP_HOST=smtp.mein-anbieter.de
VAKT_SMTP_PORT=587
VAKT_SMTP_USER=vakt@meine-firma.de
VAKT_SMTP_PASS=sicheres-passwort
VAKT_SMTP_FROM=vakt@meine-firma.de
```

---

## KI-Berater (optional)

Vakt kann Compliance-Berichte und Empfehlungen über einen OpenAI-kompatiblen KI-Provider generieren. Standardmäßig ist Ollama mit `llama3.2:3b` lokal konfiguriert — kein GPU, kein Cloud-API-Key nötig.

Unterstützte Provider: Ollama, Mistral AI, OpenAI, Groq, LM Studio, vLLM und jeder weitere OpenAI-kompatible Endpunkt.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_AI_PROVIDER` | — | `openai` | KI-Provider. `openai` aktiviert alle OpenAI-kompatiblen Endpunkte. `disabled` schaltet den Berater komplett ab. |
| `VAKT_AI_BASE_URL` | — | `http://ollama:11434/v1` | API-Basisendpunkt des Providers. |
| `VAKT_AI_API_KEY` | — | — | API-Key des Providers. Für lokale Provider (Ollama, LM Studio) leer lassen. |
| `VAKT_AI_MODEL` | — | `llama3.2:3b` | Modellname. |

**Beispiel Ollama (lokal, Standard):**

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=http://ollama:11434/v1
VAKT_AI_MODEL=llama3.2:3b
```

Modell einmalig laden:
```bash
docker compose exec ollama ollama pull llama3.2:3b
```

**Beispiel Mistral AI (EU-Server, DSGVO-freundlich):**

```env
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=https://api.mistral.ai/v1
VAKT_AI_API_KEY=sk-...
VAKT_AI_MODEL=mistral-small-latest
```

**KI deaktivieren:**

```env
VAKT_AI_PROVIDER=disabled
```

---

## Update-Benachrichtigungen (optional)

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_UPDATE_CHECK` | — | `false` | Aktiviert den täglichen Check auf neue Vakt-Versionen via GitHub Releases API. Zeigt Admins und Eigentümern ein Banner in der UI wenn eine neue Version verfügbar ist. Es werden keine Daten gesendet — nur ein lesender GET-Request an die öffentliche GitHub-API, ohne Instanz-ID oder Telemetrie. |

**Beispiel:**

```env
VAKT_UPDATE_CHECK=true
```

---

## Benutzerverwaltung & Rollen

Vakt unterscheidet zwischen der kostenlosen **Community Edition** mit vier festen Rollen und der **Pro**-Edition mit granularen Modul-Berechtigungen.

### Community-Rollen

| Rolle | Rechte |
|-------|--------|
| **Admin** | Vollzugriff — Benutzer verwalten, Module konfigurieren |
| **Analyst** | Lesen + Schreiben in allen Modulen |
| **Viewer** | Nur lesen — alle Module |
| **Auditor** | Nur lesen + Audit-Bericht exportieren |

### Pro: Granulare Modul-Berechtigungen

Mit einer Pro-Lizenz können Benutzerrechte zusätzlich auf einzelne Module eingeschränkt werden. Jeder Benutzer erhält pro Modul (Vakt Scan, Vakt Comply, Vakt Vault, Vakt Aware, Vakt Privacy) eine separate `can_read`- und `can_write`-Berechtigung.

**Verwaltung:** Einstellungen → Benutzerverwaltung → Shield-Icon neben dem jeweiligen Benutzer.

---

## OIDC / SAML Single Sign-On (optional)

SSO wird über [Casdoor](https://casdoor.org) als OIDC/SAML-Proxy unterstützt. Damit können Azure AD, Okta, Keycloak und Google Workspace eingebunden werden.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `CASDOOR_URL` | — | — | URL des Casdoor-Servers. |
| `CASDOOR_CLIENT_ID` | — | — | OAuth2 / OIDC Client-ID der Vakt-Anwendung in Casdoor. |
| `CASDOOR_CLIENT_SECRET` | — | — | OAuth2 / OIDC Client-Secret. Nicht in Git committen. |

**Beispiel:**

```env
CASDOOR_URL=https://auth.meine-firma.de
CASDOOR_CLIENT_ID=vakt-app
CASDOOR_CLIENT_SECRET=geheimes-secret
```

---

## LDAP / Active Directory (optional)

Vakt kann Benutzerkonten aus einem LDAP/AD synchronisieren.

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `VAKT_LDAP_URL` | — | — | LDAP-Server-URL. Format: `ldap://host:389` oder `ldaps://host:636`. |
| `VAKT_LDAP_BIND_DN` | — | — | Distinguished Name des Service-Accounts für die Verbindung. |
| `VAKT_LDAP_BIND_PASS` | — | — | Passwort des Service-Accounts. |
| `VAKT_LDAP_BASE_DN` | — | — | Basis-DN für die Benutzersuche. |
| `VAKT_LDAP_USER_FILTER` | — | `(objectClass=person)` | LDAP-Filter für Benutzer. |
| `VAKT_LDAP_GROUP_FILTER` | — | `(objectClass=group)` | LDAP-Filter für Gruppen. |
| `VAKT_LDAP_TLS` | — | `false` | TLS für LDAP-Verbindung aktivieren (`true`/`false`). |

**Beispiel:**

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

## Vollständige .env-Vorlage

```env
# ── Pflichtfelder ──────────────────────────────────────────────────────────────
VAKT_DB_URL=postgres://vakt:passwort@postgres:5432/vakt?sslmode=disable
VAKT_REDIS_URL=redis://redis:6379
VAKT_SECRET_KEY=<openssl rand -hex 32>

# ── Datenbank-Container ────────────────────────────────────────────────────────
POSTGRES_PASSWORD=passwort

# ── Anwendung ──────────────────────────────────────────────────────────────────
VAKT_API_PORT=8080
APP_VERSION=1.0.0
VAKT_MODULES_ENABLED=vaktscan,vaktcomply,vaktvault,vaktaware,vaktprivacy,sechr
AUTO_MIGRATE=false
VAKT_FRONTEND_URL=https://vakt.meine-firma.de
VAKT_UPLOAD_DIR=/data/uploads
# VAKT_UPDATE_CHECK=false   # opt-in: täglicher Check auf neue Versionen via GitHub API

# ── SMTP ────────────────────────────────────────────────────────────────────────
VAKT_SMTP_HOST=smtp.meine-firma.de
VAKT_SMTP_PORT=587
VAKT_SMTP_USER=vakt@meine-firma.de
VAKT_SMTP_PASS=
VAKT_SMTP_FROM=vakt@meine-firma.de

# ── KI-Berater (Ollama lokal, kein GPU nötig) ──────────────────────────────────
VAKT_AI_PROVIDER=openai
VAKT_AI_BASE_URL=http://ollama:11434/v1
VAKT_AI_API_KEY=
VAKT_AI_MODEL=llama3.2:3b

# ── OIDC / SSO (optional) ──────────────────────────────────────────────────────
CASDOOR_URL=
CASDOOR_CLIENT_ID=
CASDOOR_CLIENT_SECRET=

# ── LDAP / Active Directory (optional) ────────────────────────────────────────
VAKT_LDAP_URL=
VAKT_LDAP_BIND_DN=
VAKT_LDAP_BIND_PASS=
VAKT_LDAP_BASE_DN=
VAKT_LDAP_USER_FILTER=(objectClass=person)
VAKT_LDAP_GROUP_FILTER=(objectClass=group)
VAKT_LDAP_TLS=false
```

---

## Enterprise-Integrationen (keine Env-Vars)

Die folgenden Integrationen werden **pro Organisation** in der Vakt-Oberfläche konfiguriert (Admin → Einstellungen) und benötigen keine eigenen Umgebungsvariablen:

| Integration | Edition | Setup |
|-------------|---------|-------|
| **SAML 2.0 Direct SP** | CE | Admin → Enterprise SSO → SAML → IdP-Metadaten hochladen |
| **OIDC via Casdoor** | CE | `CASDOOR_*`-Vars (siehe oben) |
| **SCIM 2.0 Provisioning** | Pro | Admin → Enterprise SSO → SCIM → Token generieren |
| **SIEM-Forwarder** (Splunk, Elastic, Webhook) | Pro | Admin → Integrationen → SIEM → Adapter + Endpoint konfigurieren |
| **IP-Allowlist für Admin-Endpunkte** | Pro | Admin → Sicherheit → IP-Allowlist → CIDR-Einträge |
| **MFA für sensitive API-Calls** | Pro | Admin → Sicherheit → MFA-Enforcement |

Ausführliche Setup-Anleitungen: `docs/wiki/enterprise-sso.md`

---

## Hinweise

### AUTO_MIGRATE nur kontrolliert einsetzen

`AUTO_MIGRATE=true` ist praktisch für einfache Setups. In Produktionsumgebungen mit kritischen Daten empfiehlt sich:

1. Backup anlegen
2. Migration manuell prüfen und ausführen
3. Ergebnis verifizieren
4. Erst dann neuen Anwendungscode starten

### Module einzeln deaktivieren

Jedes Modul kann unabhängig deaktiviert werden. Die Modul-Namen sind case-insensitiv:

```env
# Nur Vakt Comply und Vakt Vault aktiv
VAKT_MODULES_ENABLED=vaktcomply,vaktvault
```

