# Vakt Scan — Scanner Setup

Vakt Scan orchestriert externe Scanner, dedupliziert Findings und priorisiert nach Risiko.
Drei Scanner werden unterstützt: Trivy (gebündelt), Nuclei (gebündelt) und OpenVAS (extern, optional).

## Trivy — Vulnerability-Scanner (gebündelt)

Trivy ist direkt im Docker-Image enthalten. Kein Setup erforderlich.

Trivy scannt:
- Container-Images auf bekannte CVEs (NVD, GitHub Advisory DB)
- Dateisystem-Pfade auf verwundbare Libraries
- IaC-Konfigurationen (Terraform, Kubernetes)

Der erste Scan lädt die Trivy-Datenbank herunter (~200 MB). Danach werden Updates automatisch gecacht.

**Erster Scan:**
1. Asset anlegen: Vakt Scan → Assets → "Asset anlegen"
2. Scanner auf "Trivy" setzen
3. "Scan starten" — Fortschritt läuft live via SSE
4. Findings erscheinen nach Abschluss in der Findings-Liste

## Nuclei — Template-basierter Web-Scanner (gebündelt)

Nuclei ist ebenfalls im Docker-Image enthalten. Kein Setup erforderlich.

Nuclei prüft Web-Endpoints auf bekannte Schwachstellen via Community-Templates
(OWASP Top 10, CVE-Exploits, Fehlkonfigurationen).

**Erster Scan:**
- Asset vom Typ "Web-Anwendung" anlegen
- Scanner auf "Nuclei" setzen
- Scan starten

## OpenVAS — Netzwerk-Vulnerability-Scanner (extern, optional)

OpenVAS ist ein vollständiger Netzwerk-Scanner und muss separat betrieben werden.
Er eignet sich für tiefes Netzwerk-Scanning (TCP/UDP-Ports, Service-Detection, authenticated scans).

### OpenVAS starten (Docker Compose)

```yaml
services:
  openvas:
    image: immauss/openvas:latest
    ports:
      - "9392:9392"  # Greenbone Security Assistant (Web-UI)
    volumes:
      - openvas_data:/data
    environment:
      USERNAME: admin
      PASSWORD: your-openvas-password
```

### Vakt mit OpenVAS verbinden

In `.env` eintragen:

```env
VAKT_OPENVAS_URL=http://openvas:9392
VAKT_OPENVAS_USER=admin
VAKT_OPENVAS_PASS=your-openvas-password
```

Der Vakt-API-Container muss Netzwerk-Zugriff auf den OpenVAS-Host haben.

## Scanner-Status prüfen

```bash
curl -s http://localhost/api/v1/vaktscan/scanner-status \
  -H "Authorization: Bearer <token>" | jq
# → {"trivy": true, "nuclei": true, "openvas": false}
```

`openvas: false` ist der Normalzustand wenn kein OpenVAS konfiguriert ist — Trivy und Nuclei sind ausreichend für die meisten KMU-Umgebungen.

## Troubleshooting

**"Scanner nicht eingerichtet"-Banner erscheint:**
- Prüfe ob der API-Container korrekt gestartet ist: `docker compose ps`
- Stelle sicher dass das Image aus dem aktuellen Build stammt: `docker compose pull` oder `docker compose build`
- Trivy und Nuclei sind ab v0.24.0 gebündelt — ältere Images haben sie nicht

**Erster Trivy-Scan schlägt fehl (Datenbankfehler):**
- Container braucht Internetzugang für den initialen DB-Download
- Prüfe Firewall-Regeln: `ghcr.io` und `github.com` müssen erreichbar sein
- Datenbank wird in `/tmp/trivy-cache` zwischengespeichert; persistiere diesen Pfad für Produktionsumgebungen
