# Deployment-Workflow

## Übersicht

Vakt nutzt einen zweistufigen Deploy-Prozess: Jeder Push auf `main` landet automatisch auf **Staging**, auf die **Demo** kommt nur, was explizit promoviert wurde.

```
feature-branch → PR → merge main
       ↓
CI baut :staging-Image → Runner deployed automatisch auf Staging
       ↓
Manuelle QA auf Staging (interne Testumgebung)
       ↓
"Zur Demo promoten" in Staging-Einstellungen
       ↓
:staging wird als :latest gepusht → Demo-Container neu gestartet
```

---

## Staging

**URL:** Interne Testumgebung (nicht öffentlich)

**Wann aktualisiert:** Automatisch nach jedem erfolgreichen CI-Lauf auf `main`.

**Ablauf:**
1. CI baut Backend- und Frontend-Images, tagged sie als `:staging`
2. Runner auf dem Server zieht die neuen Images
3. `staging-api`, `staging-worker` und `staging-frontend` werden neu gestartet

**Erkennungszeichen:** In den Einstellungen erscheint ein "Staging"-Abschnitt mit dem Promote-Button — auf anderen Instanzen nicht sichtbar.

---

## Demo zur Demo promoten

**Voraussetzung:** QA auf Staging abgeschlossen, grünes Licht gegeben.

**Schritte:**
1. In der Staging-Instanz → Einstellungen → Abschnitt "Staging"
2. Button "Zur Demo promoten" klicken
3. Bestätigungsdialog bestätigen
4. Die Demo wird innerhalb von ca. 2 Minuten aktualisiert

**Was im Hintergrund passiert:**
- `POST /api/v1/admin/staging/promote` triggert einen lokalen Webhook auf dem Server
- Der Webhook führt `/usr/local/bin/vakt-do-promote` aus:
  - `docker pull ghcr.io/matharnica/vakt-api:staging`
  - `docker pull ghcr.io/matharnica/vakt-frontend:staging`
  - Re-Tag beider Images als `:latest`
  - Push zu GHCR
  - `docker compose up -d` für Demo-Container
- Log: `/var/log/vakt-promote.log` auf dem Server

---

## Infrastruktur-Details

### Instanzen

| Instanz | Image-Tag | Aktualisierung |
|---------|-----------|----------------|
| Staging | `:staging` | Automatisch per CI auf jeden Push nach `main` |
| Demo | `:latest` | Manuell per Promote-Webhook |

### Relevante Systemd-Services (auf dem Deploy-Host)

```bash
# Promote-Webhook (lauscht auf 127.0.0.1:9099)
systemctl status vakt-promote.service

# GitHub Actions Self-hosted Runner (Name variiert je nach Installation)
systemctl status actions.runner.<runner-name>.service
```

### Promote-Webhook manuell testen

```bash
# Auf dem Deploy-Host
curl -X POST http://127.0.0.1:9099/promote \
  -H "X-Promote-Secret: <VAKT_PROMOTE_SECRET aus .env>"
```

### Promote-Log einsehen

```bash
ssh <deploy-host> "tail -f /var/log/vakt-promote.log"
```

---

## Wichtige Umgebungsvariablen (Staging)

| Variable | Wert | Zweck |
|----------|------|-------|
| `VAKT_STAGING` | `true` | Aktiviert Promote-Endpoint und Settings-UI |
| `VAKT_PROMOTE_URL` | `http://host.docker.internal:9099/promote` | Webhook-Adresse |
| `VAKT_PROMOTE_SECRET` | *(in .env)* | Shared Secret für den Webhook |

---

## GitHub Actions Workflows

| Workflow | Trigger | Zweck |
|----------|---------|-------|
| `ci.yml` → `deploy-staging` | Push auf `main` | Baut `:staging` und deployed auf Staging |
| `promote.yml` | Manuell via GitHub UI | Alternativer Promote-Weg (ohne Button) |
| `release.yml` | Git-Tag `v*` | Öffentliches Release für Self-Hosted-Kunden |

### Promote manuell via GitHub UI

Falls der Button nicht erreichbar ist, kann `promote.yml` direkt in GitHub Actions gestartet werden:
Actions → "Promote Staging → Demo" → "Run workflow"
