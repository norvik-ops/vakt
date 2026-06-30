# Vakt Scan

Vakt Scan orchestriert bestehende Open-Source-Scanner und normalisiert deren Ergebnisse in einem einheitlichen Finding-Modell. Duplikate werden automatisch konsolidiert, Findings nach CVSS- und EPSS-Score priorisiert und per SLA-Fristen verfolgt. Sobald ein Finding geschlossen wird, erzeugt Vakt automatisch einen Compliance-Nachweis in Vakt Comply.

Vakt Scan bringt keine eigenen Scanner mit — es orchestriert Scanner, die bereits in deiner Infrastruktur vorhanden sind oder als Container laufen.

![Vakt Scan – Vulnerability-Scanner](https://raw.githubusercontent.com/norvik-ops/vakt/main/docs/wiki/assets/screenshots/vakt-03-scan.gif)

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktcomply,vaktvault,vaktaware,vaktprivacy
```

---

## Unterstützte Scanner

| Scanner | Einsatzzweck |
|---------|--------------|
| **Trivy** | Container-Images, Dateisystem, Git-Repositories — Schwachstellen in Paketen und OS-Bibliotheken |
| **Nuclei** | Webapplikationen — OWASP-Schwachstellen, Fehlkonfigurationen, CVEs |
| **OpenVAS** | Netzwerk-Hosts — umfassendes Vulnerability Assessment |

---

## Asset-Verwaltung

Assets sind die Objekte, die gescannt werden. Typen: `server`, `container`, `webapp`, `repository`.

Jedes Asset bekommt:
- Eine Kritikalitätsstufe (low / medium / high / critical)
- Eine **Umgebung** (`prod`, `staging`, `dev`) — beeinflusst SLA-Priorisierung und Filter-Voreinstellungen
- Tags für Gruppierung und Unterdrückungsregeln
- Optional eine externe URL
- Einen **Owner** (Verantwortliche Person/Team)

Assets können einzeln angelegt oder per CSV-Massenimport hinzugefügt werden.

---

## Scans auslösen

Scans können manuell über die API oder die UI ausgelöst werden:

```
POST /api/v1/vaktscan/assets/:id/scans
Body: { "scanner": "trivy" }
```

Scans laufen asynchron. Status und Ergebnisse sind über die API abrufbar.

### Geplante Scans

Pro Asset und Scanner können wiederkehrende Scan-Schedules eingerichtet werden (Cron-Ausdruck). Beispiel: täglicher Trivy-Scan eines Containers um 02:00 Uhr.

---

## Finding-Verarbeitung

Nach einem Scan normalisiert Vakt Scan alle Ergebnisse in ein einheitliches Finding-Modell:

- **Deduplizierung** — Gleiche CVE-ID auf gleichem Asset wird über Scanner hinweg zusammengeführt; Severity wird auf Maximum gesetzt, alle Quellen (`sources`-JSONB) werden beibehalten
- **CVE-Anreicherung** — CVSS-Score und Beschreibung werden über NVD API v2.0 nachgeladen (Redis-Cache 24h; automatischer Retry bei 429-Responses)
- **EPSS-Anreicherung** — EPSS-Score (Exploit Prediction Scoring System) wird nach dem Scan automatisch nachgeladen

### Finding-Status

| Status | Bedeutung |
|--------|-----------|
| `open` | Offen, noch nicht bearbeitet |
| `in_progress` | Remediierung läuft |
| `resolved` | Behoben |
| `accepted_risk` | Risiko bewusst akzeptiert (mit Begründung) |
| `false_positive` | Als False Positive markiert |

---

## SLA-Management

Pro Schweregrad können Remediierungsfristen konfiguriert werden:

| Schweregrad | Empfohlene Frist |
|-------------|-----------------|
| Critical | 7 Tage |
| High | 30 Tage |
| Medium | 90 Tage |
| Low | 180 Tage |

Das SLA-Dashboard zeigt alle offenen Findings mit Fristenstatus — welche sind überfällig, welche laufen bald ab. Überfällige Findings erhalten in der Findings-Liste ein rotes **"SLA überfällig"**-Badge.

---

## Unterdrückungsregeln

Bekannte Findings, die bewusst ignoriert werden sollen, können dauerhaft unterdrückt werden:

- Nach CVE-ID (z. B. ein CVE, das nicht anwendbar ist)
- Nach Asset-Tag (z. B. alle Dev-Assets ausschließen)

Unterdrückte Findings tauchen im Dashboard nicht mehr auf, sind aber weiterhin abrufbar.

---

## Findings zuweisen

Jedes Finding kann einem Teammitglied zugewiesen werden. Bei Zuweisung geht eine Benachrichtigung raus. Bulk-Updates erlauben es, Status oder Zuweisung für mehrere Findings gleichzeitig zu setzen.

---

## Automatische Compliance-Evidenz

Wenn ein Finding auf `resolved` gesetzt wird, erzeugt Vakt Scan automatisch einen Compliance-Nachweis in Vakt Comply — im Patch-Management-Control des aktiven Frameworks. Damit ist jede behobene Schwachstelle direkt als Evidenz dokumentiert, ohne manuellen Aufwand.

---

## BSI CERT-Bund Advisory Feed

Vakt bezieht täglich aktuelle Sicherheitshinweise aus dem BSI CERT-Bund-Feed und zeigt sie im Dashboard an. Admininstratoren sehen so auf einen Blick, ob relevante Warnungen für die eigene Infrastruktur vorliegen.

---

## Export

Findings können als CSV oder JSON exportiert werden, gefiltert nach Schweregrad und Status. Das ist nützlich für externe Berichte oder die Übergabe an ein Ticket-System.

---

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2f | Schwachstellenmanagement und Offenlegung |
| ISO 27001:2022 A.8.8 | Management von technischen Schwachstellen |
| BSI IT-Grundschutz OPS.1.1.6 | Software-Tests; SYS.1.1 M24 Schwachstellenmanagement |

---

## Rollen

| Rolle | Rechte |
|-------|--------|
| Admin, SecurityAnalyst | Vollzugriff — Scans auslösen, Findings bearbeiten, SLA konfigurieren |
| Viewer, AuditorReadOnly | Nur lesend |

---

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `vaktscan:scan:trivy` | API / Schedule | Trivy-Scan ausführen |
| `vaktscan:scan:nuclei` | API / Schedule | Nuclei-Scan ausführen |
| `vaktscan:scan:openvas` | API / Schedule | OpenVAS-Scan ausführen |
| `vaktscan:epss_enrich` | Nach Scan | EPSS-Scores für neue Findings nachladen |
| `vaktscan:auto_evidence` | Finding-Schließung | Compliance-Nachweis in Vakt Comply erstellen |
| `vaktscan:generate_report` | API-Aufruf | Report asynchron generieren |
