# Vakt Privacy

Vakt Privacy ist der DSGVO-Dokumentations-Hub innerhalb von Vakt. Es deckt alle praxisrelevanten DSGVO-Pflichten ab: Verzeichnis der Verarbeitungstätigkeiten (Art. 30), Datenschutz-Folgeabschätzungen (Art. 35), Auftragsverarbeiterverträge (Art. 28), Datenpannenmeldungen (Art. 33/34) und Betroffenenrechts-Anfragen (Art. 15–21).

Datenpannenmeldungen werden automatisch mit dem Vorfallsregister in Vakt Comply verknüpft. Abgeschlossene Betroffenenanfragen erzeugen automatisch Compliance-Evidenz.

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktcomply,vaktscan,vaktvault,vaktaware
```

---

## VVT — Verzeichnis der Verarbeitungstätigkeiten (Art. 30 DSGVO)

Das VVT ist die Grundlage jeder DSGVO-Dokumentation. Vakt erfasst pro Verarbeitungstätigkeit:

- Zweck und Rechtsgrundlage (z. B. Art. 6 Abs. 1 lit. b)
- Datenkategorien und betroffene Personengruppen
- Empfänger der Daten
- Aufbewahrungsdauer
- Drittlandtransfer (ja/nein) und Schutzmaßnahmen

VVT-Einträge haben einen Status (active / archived). Der CSV-Export liefert das vollständige Verzeichnis zur Vorlage bei der Datenschutzbehörde.

---

## DPIA — Datenschutz-Folgeabschätzung (Art. 35 DSGVO)

Eine DPIA ist bei Verarbeitungen mit hohem Risiko für Betroffene Pflicht (z. B. Profiling, Videoüberwachung, Gesundheitsdaten).

Vakt unterstützt den gesamten DPIA-Prozess:

1. DPIA mit VVT-Eintrag verknüpfen
2. Notwendigkeits- und Verhältnismäßigkeitsprüfung dokumentieren
3. Risiken und Minderungsmaßnahmen erfassen
4. Restrisiko bewerten
5. DSB-Konsultation dokumentieren (falls erforderlich)
6. Genehmigungs-Workflow: DPIA zur Genehmigung einreichen

DPIA-Berichte können exportiert werden (PDF und JSON).

---

## AVV — Auftragsverarbeitungsverträge (Art. 28 DSGVO)

Jedes Unternehmen, das personenbezogene Daten von einem Dienstleister verarbeiten lässt, braucht einen AVV.

Vakt verwaltet AVVs mit:

- Name des Auftragsverarbeiters und Leistungsbeschreibung
- Vertragsabschlussdatum und Review-Datum
- Ablaufdatum — bei Überschreitung wechselt der Status automatisch auf `expired`
- Tägliche automatische Prüfung und Ablauf-Alerts

Damit gerät kein abgelaufener AVV in Vergessenheit.

---

## Datenpannenmeldungen (Art. 33/34 DSGVO)

Bei einer Datenpanne (Data Breach) greifen strenge Fristen: Die zuständige Aufsichtsbehörde muss innerhalb von 72 Stunden nach Bekanntwerden informiert werden.

Vakt berechnet die Meldepflicht-Frist automatisch aus dem Entdeckungszeitpunkt und stellt den Status als Ampel dar.

### Workflow

1. Datenpanne anlegen (Zeitpunkt der Entdeckung, betroffene Daten, Anzahl Betroffener)
2. 72-Stunden-Frist läuft automatisch — Ampelstatus zeigt den Handlungsbedarf
3. Meldung an die Behörde vorbereiten und als erledigt markieren
4. Falls Betroffene informiert werden müssen (Art. 34): separat dokumentieren
5. PDF-Export der Meldung für die Akten

Gleichzeitig legt Vakt automatisch einen verknüpften Eintrag im Vorfallsregister von Vakt Comply an. So ist die Datenpanne sowohl aus Privacy- als auch aus Sicherheitsperspektive dokumentiert.

---

## DSR — Betroffenenrechts-Anfragen (Art. 15–21 DSGVO)

Betroffene haben das Recht, Auskunft, Löschung, Übertragbarkeit und Widerspruch gegen die Verarbeitung ihrer Daten zu verlangen. Die Frist für eine Antwort beträgt 30 Tage (Art. 12 Abs. 3).

Vakt verwaltet Betroffenenanfragen mit:

| Typ | Rechtsgrundlage |
|-----|-----------------|
| `access` | Auskunftsrecht (Art. 15) |
| `erasure` | Löschungsrecht (Art. 17) |
| `portability` | Datenübertragbarkeit (Art. 20) |
| `objection` | Widerspruchsrecht (Art. 21) |
| `rectification` | Berichtigungsrecht (Art. 16) |

Die 30-Tage-Frist wird automatisch aus dem Eingangs-Datum berechnet. Überfällige Anfragen werden täglich geprüft und Alerts verschickt.

Bei Abschluss einer DSR erzeugt Vakt automatisch Compliance-Evidenz in Vakt Comply.

CSV-Export aller DSRs für Reporting und Audit.

---

## Compliance-Mapping

| Standard | Abdeckung |
|----------|-----------|
| DSGVO Art. 28 | Auftragsverarbeitung — AVV-Verwaltung mit Ablauf-Tracking |
| DSGVO Art. 30 | Verzeichnis der Verarbeitungstätigkeiten (VVT) |
| DSGVO Art. 33/34 | Datenpannenmeldung an Behörde und Betroffene |
| DSGVO Art. 35 | Datenschutz-Folgeabschätzung (DPIA) |
| DSGVO Art. 15–21 | Betroffenenrechte — DSR mit 30-Tage-Fristen-Tracking |
| NIS2 Art. 21 Abs. 2d | Sicherheit der Lieferkette (AVVs als Nachweis) |

---

## Hintergrund-Jobs

| Job | Zeitplan | Beschreibung |
|-----|----------|--------------|
| `vaktprivacy:avv_expiry_check` | Täglich | Abgelaufene AVVs auf `expired` setzen und Alerts versenden |
| `vaktprivacy:breach_incident_create` | Bei Breach-Erstellung | Verknüpften Vorfall in Vakt Comply anlegen |
| `vaktprivacy:dsr_overdue_check` | Täglich | Überfällige DSRs prüfen und Alerts versenden |
