# Vakt Aware

Vakt Aware ermöglicht interne Phishing-Simulationen und Micro-Trainings für Mitarbeiter. Das Reporting ist standardmäßig anonymisiert — einzelne Klickdaten werden nicht an die Unternehmensleitung weitergegeben (Betriebsrat-Modus). Abgeschlossene Trainings fließen automatisch als Compliance-Nachweis in Vakt Comply ein.

---

## Aktivierung

Das Modul ist standardmäßig aktiv. Zum Deaktivieren:

```env
VAKT_MODULES_ENABLED=secvitals,secpulse,secvault,secprivacy
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
| `VAKT_SMTP_FROM` | `secreflex@vakt.local` | Absenderadresse für Kampagnen |

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

15 vorgefertigte Vorlagen sind eingebaut (z. B. gefälschte IT-Abteilungs-E-Mail, CEO-Fraud, Paketbenachrichtigung).

---

## Betriebsrat-Modus

Ein zentrales Feature von Vakt Aware: Das Tracking ist datenschutzkonform.

Im **Betriebsrat-Modus** (Standard) werden Klick-Events nur auf Abteilungsebene aggregiert — nicht für einzelne Personen. Das bedeutet:

- Die Reporting-Ansicht zeigt "Marketing: 3 von 10 haben geklickt" — aber nicht, *wer* geklickt hat
- Einzelne Klickdaten werden nicht gespeichert
- Kein Unterlaufen von Betriebsvereinbarungen

Den Modus kann man pro Kampagne ein- oder ausschalten. Wenn ausgeschaltet: Individuelle Tracking-Daten werden gespeichert (nur mit entsprechender Betriebsvereinbarung).

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
| `secreflex:send_campaign` | Kampagnen-Launch | E-Mails an alle Zielgruppen-Empfänger versenden |
| `secreflex:training_reminder` | Täglich | Erinnerung an überfällige Trainings-Zuweisungen |
