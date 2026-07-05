# Vakt Comply

Vakt Comply ist das zentrale Modul der Plattform. Es führt durch die Implementierung von Compliance-Frameworks, verfolgt den Status einzelner Controls, verwaltet Nachweise mit Reviewer-Workflow und produziert auditreife Dokumentation. Alle anderen Module liefern ihre Ergebnisse automatisch als Compliance-Evidenz in Vakt Comply ein.

![Vakt Comply – Compliance-Hub](https://raw.githubusercontent.com/norvik-ops/vakt/main/docs/wiki/assets/screenshots/vakt-02-comply.gif)

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktscan,vaktvault,vaktaware,vaktprivacy
```

---

## Unterstützte Frameworks

| Framework | Abdeckung |
|-----------|-----------|
| **NIS2** (EU 2022/2555) | Alle Maßnahmen Art. 21 Abs. 2 (a–j); Meldepflicht-Assistent mit T+24h/72h/30d-Fristen |
| **ISO 27001:2022** | Alle 93 Annex-A-Controls trackbar |
| **BSI IT-Grundschutz** | 38 Bausteine als Framework abbildbar |
| **DORA** (EU 2022/2554) | Kapitel II–VI; IKT-Vorfallsregister mit 4h/24h/72h/30d-Fristen; Drittanbieter-Register |
| **TISAX** (VDA ISA) | Kapitel 1–15; Schutzbedarf Normal/Hoch/Sehr hoch; Reifegradskala 0–3 |
| **EU AI Act** (2024/1689) | KI-System-Inventar; Risikoklassen; technische Dokumentation nach Art. 11 / Annex IV |
| **DSGVO Art. 32 TOM** | 13 TOMs mit ISO-27001-Deckungsanalyse |
| **ISO 42001** | KI-Management-System-Framework |

| **CRA** (Cyber Resilience Act) | Controls für Hersteller von Produkten mit digitalen Elementen |
| **CIS Controls v8** | 61 IG1-Safeguards in 18 Kontrollgruppen mit CIS ↔ ISO 27001 Mapping |
| **ISO 27017:2015** | Cloud-Security-Controls für CSP & CSC; 31 Controls inkl. CLD-Erweiterungen; Mapping ↔ ISO 27001/C5/BSI |
| **ISO 27018:2019** | PII-Schutz in Public Clouds (Art.-28-DSGVO-Auftragsverarbeiter); Mapping ↔ DSGVO-TOM/C5/ISO 27001 |

> **Lizenz-Hinweis:** NIS2, ISO 27001, DSGVO-TOM, CIS, KRITIS und C5 sind in der Community
> Edition enthalten. Der BSI-IT-Grundschutz-Workflow (Modellierung, Grundschutz-Check,
> Cockpit, Referenzberichte), EU AI Act und CRA erfordern **Pro**. Details: [FAQ → Editionen](../faq.md).
>
> **ISO 27017 und ISO 27018** sind im Code vorhanden, werden aber aktuell nicht öffentlich angeboten.

Mehrere Frameworks können gleichzeitig aktiv sein. Vakt zeigt pro Framework einen Readiness-Score und eine Gap-Analyse.

---

## Controls tracken

Jedes Framework besteht aus Controls — einzelnen Anforderungen, die umgesetzt werden müssen. Jedes Control hat einen Status:

| Status | Bedeutung |
|--------|-----------|
| `covered` | Nachweis vorhanden und genehmigt |
| `partial` | Teilweise abgedeckt |
| `missing` | Kein Nachweis vorhanden |
| `in_progress` | Umsetzung läuft |
| `implemented` | Manuell als umgesetzt markiert |
| `not_applicable` | Nicht relevant für diese Organisation |

Pro Control können beliebig viele Implementierungsaufgaben angelegt und abgehakt werden.

### Control Approvals

Optional kann das Vier-Augen-Prinzip für Control-Statusänderungen aktiviert werden (Org-Setting unter Einstellungen). Statusänderungen müssen dann von einem zweiten Benutzer genehmigt werden, bevor sie wirksam werden.

---

## Nachweise (Evidence)

Compliance-Evidenz kann auf vier Wegen in Vakt Comply gelangen:

1. **Manuell** — Text-Nachweis direkt eingeben (z. B. Link zu einem Dokument, Beschreibung einer Maßnahme).
2. **Datei-Upload** — PDF, Screenshot oder Exportdatei hochladen.
3. **Automatisch via Collector** — GitHub, AWS, Azure oder Active Directory als Datenquelle verbinden; Vakt sammelt Evidence selbst ein.
4. **Automatisch aus anderen Modulen** — Geschlossene Findings aus Vakt Scan und abgeschlossene Trainings aus Vakt Aware werden automatisch als Nachweis übertragen.

Jede Evidence hat ein optionales Ablaufdatum. 30 Tage vor Ablauf verschickt Vakt eine Benachrichtigung.

### Reviewer-Workflow

Evidence kann einem Reviewer zugewiesen werden. Der Reviewer genehmigt oder lehnt ab (mit Begründung). Nur genehmigte Evidence zählt für den Readiness-Score.

---

## Gap-Analyse

Die Gap-Analyse zeigt, welche Controls noch nicht abgedeckt sind und warum:

- `no_evidence` — Kein Nachweis vorhanden
- `evidence_expiring` — Nachweis läuft bald ab
- `review_pending` — Nachweis wartet auf Genehmigung

Controls lassen sich nach Status filtern und sortieren. Der Export als ZIP enthält die rohen Control- und Evidence-Daten.

---

## Risikobewertung

Das Risikoregister verwaltet Risiken mit:

- **Eintrittswahrscheinlichkeit** (1–5) × **Schadensausmaß** (1–5) = Risiko-Score
- **Behandlungsstrategie**: avoid / mitigate / transfer / accept
- **Verknüpfung mit Controls** — Risiken können mit den Controls verbunden werden, die sie adressieren

Der Risk-Score hilft dabei zu priorisieren, welche Controls zuerst umgesetzt werden sollten.

---

## Vorfallsmanagement

Das Vorfallsregister dokumentiert Sicherheitsvorfälle mit Schweregrad, Status und betroffenen Systemen.

### NIS2-Meldungsassistent

Beim Anlegen eines Vorfalls führt ein Kurzfragebogen durch die Meldepflicht-Klassifizierung nach NIS2 Art. 23. Wenn Meldepflicht besteht, berechnet Vakt automatisch die Fristen:

- **T+24h** — Frühwarnung an die Behörde
- **T+72h** — Vollständige Meldung
- **T+30d** — Abschlussbericht

Der Frist-Status wird als Ampel dargestellt. 12 Stunden vor Ablauf einer Frist geht eine E-Mail-Benachrichtigung raus.

### DORA-Meldepflichten

Für IKT-Vorfälle nach DORA Art. 19 gelten engere Fristen (T+4h/24h/72h/30d). Vakt generiert vorausgefüllte Meldungsformulare im BaFin-Layout als PDF und JSON.

### Behörden-Verzeichnis

Vakt kennt die zuständigen Behörden je nach Sektor der Organisation (BSI, BaFin, BNetzA, Luftfahrtbundesamt) und wählt automatisch die richtige Anlaufstelle aus.

---

## Richtlinien-Management

Vakt kommt mit 10 deutschen Richtlinien-Vorlagen:

- Informationssicherheitsrichtlinie (ISMS)
- Passwort-Richtlinie
- Richtlinie zur akzeptablen Nutzung
- Homeoffice- und Fernarbeitsrichtlinie
- Datenklassifizierungsrichtlinie
- Incident-Response-Richtlinie
- Änderungsmanagement-Richtlinie
- Zugangs- und Zugriffskontrollrichtlinie
- Datensicherungsrichtlinie
- Lieferanten- und Dienstleistersicherheit

Richtlinien durchlaufen einen Status-Zyklus: Draft → Active → Archived. Jede Änderung erzeugt eine neue Version. Policy-Akzeptanz durch Mitarbeiter kann per E-Mail-Link eingeholt werden.

---

## Interne Audits

Interne Audit-Records dokumentieren:

- Scope und Auditor
- Befunde mit Schweregrad
- Empfehlungen und Maßnahmen

---

## Zertifizierungs-Timeline

Countdown-Karten und Kalender-Ansicht für Audit-Meilensteine (z. B. geplantes Zertifizierungsaudit, Überwachungsaudit). Fälligkeitsdaten werden im Dashboard hervorgehoben.

---

## Score-Verlauf

Der Compliance-Score wird über Zeit aufgezeichnet und als Recharts-Diagramm im Dashboard dargestellt. So ist der Fortschritt von Woche zu Woche nachvollziehbar.

---

## Auditor-Portal

Externe Prüfer können ohne eigenen Vakt-Account auf einen zeitlich begrenzten, schreibgeschützten Framework-Report zugreifen. Der Zugangslink wird in Vakt erzeugt, gilt für einen konfigurierten Zeitraum und kann jederzeit widerrufen werden.

---

## Audit-Paket Export

Für die Übergabe an externe Prüfer oder zur Archivierung: ZIP-Download mit `control.json` und `evidence.json` pro Control oder als vollständiges Bundle.

---

## Lieferanten-Portal (Supply Chain Compliance)

Vakt Comply verwaltet Lieferanten mit Kritikalitätsstufe (kritisch / wesentlich / standard) und NIS2/DORA-Relevanz-Markierung. Lieferanten können per tokenbasiertem Portal (ohne Vakt-Account) Fragebögen beantworten und Zertifikate hochladen.

Der Fragebogen-Builder unterstützt: Ja/Nein, Multiple Choice, Freitext, Datei-Upload. Vorgefertigte Vorlagen für NIS2, DORA und ISO 27001 sind enthalten.

---

## TISAX-spezifische Ansichten

- Schutzbedarf-Tabs: Normal / Hoch / Sehr hoch
- Reifegradskala 0–3 pro Control
- TISAX ↔ ISO 27001 Mapping-Tabelle mit Coverage-Berechnung
- TISAX-Bereitschaftsbericht als PDF-Export

---

## EU AI Act — KI-System-Compliance

- **KI-System-Inventar** — Name, Anbieter, Einsatzbereich, Entscheidungsautonomie, Status
- **Risiko-Klassifizierungs-Wizard** — Entscheidungsbaum nach Annex III (Verbote → Hochrisiko → Transparenzpflicht)
- **Technische Dokumentation** — Template nach Art. 11 / Annex IV mit PDF-Export und Versionierung

---

## KI-Berater

Der integrierte KI-Berater analysiert die aktuellen Compliance-Lücken der Organisation und beantwortet Fragen wie "Was sollten wir diese Woche priorisieren?" — bezogen auf die eigenen Daten, lokal auf dem eigenen Server.

Standard: Ollama mit `qwen2.5:7b` (CPU-only, kein GPU nötig, braucht 8 GB RAM; auf kleinen VMs `qwen2.5:3b`). Alternativ: jeder OpenAI-kompatible Endpunkt (Mistral, OpenAI, Groq, LM Studio).

KI komplett deaktivieren: `VAKT_AI_PROVIDER=disabled`

---

## Hintergrund-Jobs

| Job | Zeitplan | Beschreibung |
|-----|----------|--------------|
| `vaktcomply:evidence_expiry_alert` | Täglich | Warnung 30 Tage vor Ablauf von Evidence |
| `vaktcomply:incident_deadline_check` | Stündlich | Prüft NIS2/DORA-Meldefristen; E-Mail 12h vor Ablauf |
| `vaktcomply:supplier_cert_expiry` | Täglich | Warnung bei Lieferanten-Zertifikaten, die in 30 Tagen ablaufen |
