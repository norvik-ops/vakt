# Demo-Modus

Vakt kommt mit einem **Demo-Modus**, der eine vollständige Instanz mit realistischen Beispieldaten startet — ideal für Evaluierungen, interne Vorführungen oder eine öffentliche „Try it"-Variante.

---

## Quickstart

```bash
git clone https://github.com/norvik-ops/vakt.git
cd vakt
cp .env.example .env

# Demo-Modus aktivieren
echo "VAKT_DEMO=true" >> .env

./scripts/install.sh
```

Vakt startet auf `http://localhost`. Beim Aufruf wird ein **ephemeres Demo-Konto** angelegt:

- E-Mail: angezeigt im Startup-Log (z.B. `admin-demo-7b3e@vakt.local`)
- Passwort: angezeigt im Startup-Log (zufällig generiert)
- Organisation: Beispiel-Firma mit ~50 Findings, 20 Risks, 3 Frameworks (NIS2, ISO 27001, BSI), 12 VVT-Einträgen

Die Login-Daten werden **nicht** persistent gespeichert — beim nächsten Container-Restart werden sie neu generiert.

---

## Was enthält der Demo-Datensatz?

| Modul | Beispieldaten |
|-------|---------------|
| Vakt Comply | NIS2/ISO/BSI-Frameworks, 60 Controls in verschiedenen Status, gemischte Evidenzen |
| Vakt Scan | 3 Beispiel-Assets, 50 simulierte Findings (Severities low–critical) |
| Vakt Vault | 2 Beispiel-Projekte mit Secrets |
| Vakt Aware | 1 abgeschlossene Phishing-Kampagne, 3 Trainings-Module |
| Vakt Privacy | 12 VVT-Einträge, 2 DPIAs, 3 AVVs, 1 Datenpanne, 2 DSR-Anträge |
| Vakt HR | 8 Mitarbeiter, 2 Onboarding- und 1 Offboarding-Checkliste |

Alle Daten sind **fiktiv** — keine echten Personen, Unternehmen oder Sicherheitsereignisse.

---

## Öffentliche Demo-Instanz hosten

Wenn du eine öffentlich zugängliche Demo betreiben willst (z.B. `demo.deine-firma.de`), beachte:

### Sicherheits-Vorkehrungen

1. **Reverse-Proxy mit Rate-Limiting** — Demo-Instanzen werden gerne als Probier-Sandbox missbraucht. Caddy- oder nginx-Layer mit 60 req/min pro IP davor.
2. **Daten täglich zurücksetzen** — Cron-Job, der den DB-Container neu erstellt:
   ```bash
   # /etc/cron.daily/vakt-demo-reset
   docker compose -f /opt/vakt/docker-compose.yml down -v
   docker compose -f /opt/vakt/docker-compose.yml up -d
   ```
3. **Kein SMTP-Versand** — Demo-Instanz auf `VAKT_SMTP_HOST=mailpit` lassen (intern), damit Phishing-Templates nicht versehentlich an reale Adressen rausgehen.
4. **Pro-Features deaktivieren** — `VAKT_LICENSE_KEY=` leer lassen, damit Demo-User Pro-Features nicht ausprobieren und sich wundern, dass sie sie selbst hosten nicht haben.

### Sandbox-Hinweis

Setze `VAKT_DEMO_BANNER_TEXT` in der `.env`:

```env
VAKT_DEMO=true
VAKT_DEMO_BANNER_TEXT=Demo-Modus — Daten werden täglich zurückgesetzt
```

Der Text wird oben in jeder Seite als Banner angezeigt.

---

## Pro Visitor — Ephemere Demo (Default-Verhalten)

Im Demo-Modus (`VAKT_DEMO=true`) wird **bei jedem Aufruf der Login-Seite automatisch eine eigene Demo-Organisation** für den Besucher erzeugt. Mehrere Personen können parallel testen, ohne sich gegenseitig zu sehen.

**Flow:**
1. Frontend macht `POST /api/v1/demo/start` beim Mount der Login-Page.
2. Backend legt eine Org mit Random-Slug (`demo-a3f2b1c9`) an, mit:
   - Admin-User: `admin@demo-a3f2b1c9.demo`, **Random 16-hex-char Passwort**
   - Analyst-User: `analyst@demo-a3f2b1c9.demo`, **Random 16-hex-char Passwort**
3. Response: `{admin_email, admin_password, analyst_email, analyst_password, expires_in: 14400}`. Klartext-Passwörter kommen genau einmal über die Leitung — in der DB existieren nur Bcrypt-Hashes (Cost 12).
4. Frontend befüllt die Login-Form damit, Visitor klickt sich rein.
5. Cleanup-Job löscht die Org **nach 4 Stunden** (`internal/shared/demo/cleanup.go`).

**Was Demo-Credentials NICHT sind:**
- `admin@vakt.local / admin1234` — das war ein alter statischer Seed (9 Zeichen, < Mindestlänge 10)
- Irgendein anderer hardcoded Default

Die Credentials erscheinen **immer dynamisch** in der UI / im API-Response von `/demo/start`. Wer in Doku oder Tutorials feste Demo-Credentials nennt, dokumentiert einen Bug. Hintergrund + Design-Entscheidung: ADR-0015 (Entwicklungs-Repo).

**Schutzmechanismen:**
- `POST /api/v1/demo/start` ist rate-limited auf 5 Requests/Minute pro IP (Burst 5), um DB-Flooding zu verhindern.
- Cleanup-Job läuft alle 15 Min, löscht Demo-Orgs deren `created_at < NOW() - INTERVAL '4 hours'`. Effektive Lebensdauer einer Demo-Session: zwischen 4 h und 4 h 15 min.
- Demo-Modus deaktiviert keine Produktiv-Features im Code — die Trennung passiert ausschließlich über `cfg.DemoSeed`. Niemals `VAKT_DEMO=true` auf Produktiv-Instanzen mit echten Compliance-Daten.

---

## Demo-Modus deaktivieren

In Production immer:

```env
VAKT_DEMO=false
# oder Variable einfach weglassen
```

Im Demo-Modus werden zusätzliche Warn-Logs ausgegeben („**Demo mode is enabled** — do not use this instance with real data"), die in Production stören würden.

---

## Daten-Reset manuell

```bash
# Komplette DB löschen und Demo-Daten neu seeden
docker compose down -v
docker compose up -d
```

Achtung: `-v` löscht auch das `redis_data`-Volume — alle Sessions sind danach ungültig, alle aktiven Logins müssen neu erfolgen.

---

## API

```bash
# Eine eigene ephemere Demo-Org starten:
curl -X POST https://demo.deine-firma.de/api/v1/demo/start

# Antwort:
{
  "admin_email":      "admin@demo-a3f2b1c9.demo",
  "admin_password":   "<16 hex chars, einmalig>",
  "analyst_email":    "analyst@demo-a3f2b1c9.demo",
  "analyst_password": "<16 hex chars, einmalig>",
  "expires_in":       14400
}
```

Anschließend `POST /api/v1/auth/login` mit den zurückgegebenen Credentials.
