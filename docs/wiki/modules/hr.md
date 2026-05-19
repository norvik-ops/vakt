# Vakt HR

Vakt HR verwaltet den Mitarbeiter-Lebenszyklus aus Security-Perspektive: Onboarding und Offboarding mit strukturierten Checklisten, ein Mitarbeiterverzeichnis mit Statusverfolgung und automatische Compliance-Evidenz für Audits. Abgeschlossene Checklisten-Runs fließen direkt als Nachweis in Vakt Comply ein.

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secvitals,secpulse,secvault,secreflex,secprivacy
```

---

## Mitarbeiterverzeichnis

Vakt HR führt ein internes Verzeichnis aller Mitarbeitenden mit Security-relevantem Status:

| Status | Bedeutung |
|--------|-----------|
| `active` | Aktives Beschäftigungsverhältnis |
| `offboarding` | Offboarding läuft — Zugänge werden entzogen |
| `terminated` | Beschäftigung beendet, alle Zugänge widerrufen |

Das Verzeichnis ist nicht als vollständiges HR-System gedacht, sondern als Nachweis gegenüber Auditoren, dass Zugangsprovisioning und -entzug systematisch verwaltet werden.

---

## Checklisten-Vorlagen

Checklisten definieren die Schritte, die beim Onboarding oder Offboarding eines Mitarbeitenden durchgeführt werden müssen — z. B. Account-Erstellung, Gerätezuweisung, Datenschutzunterweisung oder Zugangsentzug zu kritischen Systemen.

| Typ | Einsatz |
|-----|---------|
| `onboarding` | Neue Mitarbeitende — Schritte bis zur vollständigen Zugangsprovisionierung |
| `offboarding` | Ausscheidende Mitarbeitende — Zugangsentzug, Geräterückgabe, Datenlöschung |

Vorlagen können beliebig oft wiederverwendet werden. Schritte lassen sich als verpflichtend (`required`) markieren — ein Run kann erst abgeschlossen werden, wenn alle Pflichtschritte erledigt sind.

---

## Checklisten-Runs

Ein Checklisten-Run ist eine konkrete Ausführung einer Vorlage für einen bestimmten Mitarbeitenden. Jeder Schritt wird einzeln abgehakt und mit Timestamp protokolliert — so ist nachvollziehbar, wer wann welchen Schritt abgeschlossen hat.

### Run starten

1. Mitarbeitenden im Verzeichnis auswählen
2. „Neuer Run" → Vorlage wählen
3. Schritte der Reihe nach abhaken
4. Run abschließen — Status wechselt auf `completed`

Abgeschlossene Runs sind unveränderlich und dienen als Audit-Nachweis.

---

## Typischer Offboarding-Ablauf

1. Mitarbeiterstatus auf `offboarding` setzen
2. Offboarding-Checkliste starten
3. Schritte abarbeiten: Zugänge entziehen, Gerät zurückfordern, Konten sperren
4. Run abschließen → Status wechselt auf `terminated`
5. Compliance-Evidenz wird automatisch in Vakt Comply angelegt

---

## Compliance-Integration

Abgeschlossene Checklisten-Runs erzeugen automatisch einen Nachweis in Vakt Comply (Evidenz-Typ `hr_checklist_completed`) mit Mitarbeitername, Checklisten-Name, Abschlusszeitpunkt und durchgeführten Schritten.

Diese Evidenz lässt sich direkt mit Controls verknüpfen, die Personalsicherheit verlangen — typischerweise:

| Framework | Control |
|-----------|---------|
| ISO 27001:2022 | A.6 Personalsicherheit |
| BSI IT-Grundschutz | ORP.2 Personal |
| NIS2 | Art. 21 Abs. 2 (i) — Personalsicherheit |

Auditoren sehen auf einen Blick, dass für jeden Mitarbeitenden ein vollständig dokumentierter Onboarding- und Offboarding-Prozess durchgeführt wurde.
