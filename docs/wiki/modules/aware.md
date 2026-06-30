# Vakt Aware

Vakt Aware ermöglicht interne Phishing-Simulationen und Micro-Trainings für Mitarbeiter. Das Reporting ist standardmäßig anonymisiert — einzelne Klickdaten werden nicht an die Unternehmensleitung weitergegeben (Betriebsrat-Modus). Abgeschlossene Trainings fließen automatisch als Compliance-Nachweis in Vakt Comply ein.

![Vakt Aware – Awareness-Training](https://raw.githubusercontent.com/norvik-ops/vakt/main/docs/wiki/assets/screenshots/vakt-04-aware.gif)

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=vaktcomply,vaktscan,vaktvault,vaktprivacy
```

---

## Konfiguration

Vakt Aware benötigt einen SMTP-Server, um Phishing-Simulations-E-Mails zu versenden.

| Variable | Standard | Beschreibung |
|----------|----------|--------------|
| `VAKT_SMTP_HOST` | `localhost` | SMTP-Server-Hostname |
| `VAKT_SMTP_PORT` | `1025` | Port — `1025` für Mailpit (Entwicklung), `587` für STARTTLS, `465` für SSL |
| `VAKT_SMTP_USER` | — | Benutzername (erforderlich für Port 587/465) |
| `VAKT_SMTP_PASS` | — | Passwort (erforderlich für Port 587/465) |
| `VAKT_SMTP_FROM` | `vaktaware@vakt.local` | Absenderadresse für Kampagnen |

Für lokale Entwicklung und Tests ist [Mailpit](https://github.com/axllent/mailpit) bereits in der Dev-Compose-Konfiguration enthalten.

---

## Phishing-Simulationen

### Wie es funktioniert

1. Eine E-Mail-Vorlage wird ausgewählt oder erstellt
2. Eine Zielgruppe (Gruppe von Empfängern) wird zugewiesen
3. Optional eine Landing Page konfigurieren — die Seite, die nach dem Klick erscheint
4. Kampagne starten — Vakt verschickt die E-Mails über SMTP
5. Events werden aufgezeichnet: Öffnungen, Klicks, Formular-Eingaben

### Angriffstypen

| Typ | Beschreibung |
|-----|--------------|
| `phishing` | E-Mail-basierter Angriff (der häufigste Typ) |
| `vishing` | Voice-Phishing-Simulation |
| `usb` | USB-Drop-Angriffssimulation |
| `smishing` | SMS-basierter Angriff |

10+ vorgefertigte DACH-spezifische Vorlagen sind eingebaut — alle in Deutsch, mit realistischen Mustern aus BSI-/CERT-Bund-Phishing-Reports. Abrufbar über `GET /api/v1/vaktaware/templates/presets`. Beispiele: CEO-Fraud (deutsche Anrede), IT-Helpdesk Passwort-Reset, DHL-Paket-Zustellung, Microsoft-365-MFA-Warnung, Mahnung-PDF, OneDrive-Share, Sparkasse-SMS, USB-Köder-Szenario.

Zusätzlich verfügbar: 5+ vorgefertigte **Trainings-Module** (`GET /api/v1/vaktaware/training-modules/presets`) — Phishing-Grundlagen, MFA-Aufklärung, Smishing, USB-Köder, Vishing.

---

## Betriebsrat-Modus — Anonymisierungs-Garantie

Ein zentrales Feature von Vakt Aware: Das Tracking ist DSGVO- und §87-BetrVG-konform.

### Was im Betriebsrat-Modus passiert

Wenn eine Kampagne mit `betriebsrat_mode = true` läuft, garantiert Vakt Aware **technisch erzwungen** (nicht nur per Display-Filter):

| Datentyp | Bei `betriebsrat_mode=true` |
|----------|------------------------------|
| IP-Adresse des Klicks | **Wird nicht gespeichert** (leer in der DB) |
| User-Agent / Browser | **Wird nicht gespeichert** (leer in der DB) |
| `target_id` (Person) | **Nicht im Event gespeichert** |
| Department-Aggregat | Wird gespeichert (für Abteilungsstatistiken) |
| Klick / Open / Submission | Wird als Vorgang gezählt (ohne Personenbezug) |
| Trainings-Completions | Werden auf Department-Ebene gezählt |

Die Anonymisierung greift **beim Schreiben** in die DB — eine spätere Modus-Umstellung kann keine Daten zurückholen, die nie gespeichert wurden. Das ist datenschutzrechtlich belastbarer als reine Display-Filter, weil keine "Schattenhistorie" existiert.

### Was die Reporting-Ansicht zeigt

- ✅ "Marketing: 3 von 10 haben geklickt"
- ✅ "Klickrate über alle Abteilungen: 17 %"
- ❌ Welcher Mitarbeiter geklickt hat
- ❌ Welche IP / welcher Browser

### Wann sollte der Modus ausgeschaltet werden?

Nur mit ausdrücklicher **Betriebsvereinbarung** und schriftlicher Zustimmung des Betriebsrats. Häufige Anwendungsfälle:
- Kleine Unternehmen ohne Betriebsrat (Geschäftsführer-Entscheidung)
- Konzerne mit konzernweiter "Security-Awareness-Vereinbarung", die personenbezogenes Tracking explizit erlaubt
- Pilotphase mit Freiwilligen, die schriftlich eingewilligt haben

### Compliance-Begründung

| Rechtsgrundlage | Begründung |
|-----------------|-----------|
| DSGVO Art. 5 (1c) — Datenminimierung | Es werden nur Daten erhoben, die zur Statistik nötig sind |
| DSGVO Art. 32 — TOM | Anonymisierung als technisch-organisatorische Maßnahme |
| §87 BetrVG Abs. 1 Nr. 6 | Mitbestimmung bei technischen Überwachungseinrichtungen — kein Personenbezug = keine Überwachung |
| ISO 27001 A.6.3 | Information Security Awareness, Education and Training — DSGVO-konform umsetzbar |

---

## Zielgruppen

Empfänger werden in benannten Gruppen organisiert. Jede Gruppe kann per CSV-Massenimport befüllt werden. Alternativ kann Vakt Aware aus einem verbundenen Active Directory (via LDAP) synchronisieren.

---

## Trainingsmodule

Nach einer Phishing-Simulation werden Mitarbeitern, die auf den Link geklickt haben, automatisch passende Trainingsmodule zugewiesen.

Module können sein:
- **Video** — Link zu einem Lernvideo
- **Quiz** — Fragen mit Antwortoptionen und konfigurierbarer Bestehensgrenze (1–100 %)

Vakt Aware erinnert automatisch an überfällige Trainings-Zuweisungen.

---

## Automatische Compliance-Evidenz

Wenn ein Mitarbeiter ein Training abschließt (Quiz bestanden), erzeugt Vakt automatisch einen Compliance-Nachweis in Vakt Comply — im Awareness-und-Schulungs-Control des aktiven Frameworks.

Das bedeutet: Jede abgeschlossene Schulungsrunde ist automatisch als Evidenz für ISO 27001 A.6.3, NIS2 Art. 21 Abs. 2g oder BSI ORP.3 dokumentiert.

---

## Kampagnen-Statistiken

Pro Kampagne werden folgende Kennzahlen angezeigt:

| Metrik | Beschreibung |
|--------|--------------|
| `total_targets` | Anzahl angeschriebener Empfänger |
| `emails_sent` | Tatsächlich versendete E-Mails |
| `open_rate` | Anteil geöffneter E-Mails |
| `click_rate` | Anteil geklickter Links |
| `submission_rate` | Anteil eingereichter Formulare (Credential-Eingaben) |

---

## Wiederkehrende Kampagnen

Kampagnen können als einmalig, monatlich oder quartalsweise konfiguriert werden. Vakt plant die nächste Ausführung automatisch.

---

## Compliance-Mapping

| Standard | Control |
|----------|---------|
| NIS2 Art. 21 Abs. 2g | Schulungen zur Cybersicherheit und Grundhygiene |
| ISO 27001:2022 A.6.3 | Sicherheitsbewusstsein, Aus- und Weiterbildung |
| BSI IT-Grundschutz ORP.3 | Sensibilisierung und Schulung |

---

## Rollen

| Rolle | Rechte |
|-------|--------|
| Admin, SecurityAnalyst | Vollzugriff — Kampagnen anlegen und starten, Trainings konfigurieren |
| Viewer, AuditorReadOnly | Nur lesend |

---

## Hintergrund-Jobs

| Job | Auslöser | Beschreibung |
|-----|----------|--------------|
| `vaktaware:send_campaign` | Kampagnen-Launch | E-Mails an alle Zielgruppen-Empfänger versenden |
| `vaktaware:training_reminder` | Täglich | Erinnerung an überfällige Trainings-Zuweisungen |
