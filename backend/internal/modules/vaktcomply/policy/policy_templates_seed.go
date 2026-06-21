// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPolicyTemplate mirrors the ck_policy_templates DB row returned by seed/list queries.
type DBPolicyTemplate struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	Framework   *string  `json:"framework,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

type seedTemplate struct {
	category    string
	name        string
	description string
	content     string
	tags        []string
	framework   *string
}

func strPtr(s string) *string { return &s }

var builtinDBTemplates = []seedTemplate{
	// --- Policies ---
	{
		category: "policy",
		name:     "Informationssicherheitsrichtlinie",
		description: "Grundlegende Richtlinie für das Informationssicherheitsmanagementsystem (ISMS) der Organisation " +
			"gemäß ISO 27001 und NIS2.",
		content: `# Informationssicherheitsrichtlinie

## 1. Zweck und Geltungsbereich

Diese Richtlinie legt die grundlegenden Anforderungen an die Informationssicherheit innerhalb der Organisation fest. Sie gilt für alle Mitarbeiter, externen Dienstleister und Systeme, die auf Informationen der Organisation zugreifen oder diese verarbeiten.

Die Geschäftsleitung bekennt sich ausdrücklich zum Schutz der Vertraulichkeit, Integrität und Verfügbarkeit aller Informationswerte und stellt die notwendigen Ressourcen für ein wirksames Informationssicherheitsmanagementsystem (ISMS) bereit.

## 2. Grundsätze der Informationssicherheit

Die Organisation verpflichtet sich zu folgenden Grundsätzen:

- **Vertraulichkeit**: Informationen werden nur autorisierten Personen zugänglich gemacht.
- **Integrität**: Die Vollständigkeit und Richtigkeit von Informationen wird sichergestellt.
- **Verfügbarkeit**: Informationen und Systeme sind für autorisierte Nutzer zum benötigten Zeitpunkt verfügbar.
- **Rechtskonformität**: Alle gesetzlichen, regulatorischen und vertraglichen Anforderungen werden eingehalten.

## 3. Verantwortlichkeiten

- **Geschäftsführung**: Bereitstellung von Ressourcen, strategische Ausrichtung des ISMS, Vorbildfunktion.
- **IT-Sicherheitsbeauftragter (ISB)**: Koordination, Überwachung und kontinuierliche Verbesserung des ISMS.
- **Führungskräfte**: Umsetzung der Richtlinien in ihren Bereichen, Sensibilisierung der Mitarbeiter.
- **Mitarbeiter**: Einhaltung aller Sicherheitsrichtlinien, Meldung von Vorfällen und Schwachstellen.
- **Externe Dienstleister**: Einhaltung der für sie geltenden Sicherheitsanforderungen (vertraglich geregelt).

## 4. Risikomanagement

Die Organisation identifiziert, bewertet und behandelt Informationssicherheitsrisiken systematisch. Risikoakzeptanzkriterien werden von der Geschäftsführung festgelegt. Alle identifizierten Risiken werden im Risikoregister dokumentiert und regelmäßig überprüft.

## 5. Schulung und Sensibilisierung

Alle Mitarbeiter werden bei Eintritt in die Organisation und danach jährlich zu Informationssicherheitsthemen geschult. Die Teilnahme ist verpflichtend und wird dokumentiert.

## 6. Überprüfung und Verbesserung

Diese Richtlinie wird mindestens jährlich oder bei wesentlichen Änderungen der Rahmenbedingungen überprüft und bei Bedarf aktualisiert. Das Management Review umfasst die Bewertung der Wirksamkeit des ISMS.`,
		tags:      []string{"iso27001", "nis2"},
		framework: strPtr("ISO 27001"),
	},
	{
		category: "policy",
		name:     "Passwort- und Zugangskontrollrichtlinie",
		description: "Regelungen für sichere Passwörter, Multi-Faktor-Authentifizierung und Verwaltung von " +
			"Zugriffsrechten gemäß ISO 27001 Annex A.",
		content: `# Passwort- und Zugangskontrollrichtlinie

## 1. Geltungsbereich

Diese Richtlinie gilt für alle Benutzerkonten auf Systemen, Anwendungen und Diensten der Organisation, einschließlich Cloud-Diensten und Remote-Zugriffen.

## 2. Anforderungen an Passwörter

Für alle Benutzerkonten gelten folgende Mindestanforderungen:

- Mindestlänge: 12 Zeichen (privilegierte Konten: 16 Zeichen)
- Kombination aus Groß- und Kleinbuchstaben, Ziffern und Sonderzeichen
- Keine vollständigen Wörterbuchbegriffe oder vorhersehbare Muster
- Kein Bezug zu persönlichen Daten (Name, Geburtsdatum, Unternehmensname)
- Passwörter dürfen nicht für mehrere Dienste wiederverwendet werden

## 3. Passwort-Management

- Passwörter sind geheim zu halten und dürfen nicht schriftlich notiert oder unverschlüsselt gespeichert werden.
- Die Nutzung eines von der IT genehmigten Passwort-Managers ist empfohlen.
- Passwörter dürfen niemals per E-Mail oder Messenger übermittelt werden.
- Bei Verdacht auf Kompromittierung ist das Passwort unverzüglich zu ändern und der ISB zu informieren.

## 4. Multi-Faktor-Authentifizierung (MFA)

MFA ist verpflichtend für:

- Alle administrativen und privilegierten Konten
- Remote-Zugriffe auf interne Ressourcen (VPN, RDP, SSH)
- Zugriff auf Systeme mit besonders schützenswerten Daten
- Cloud-Management-Konsolen

## 5. Berechtigungsvergabe und -entzug

- Zugriffsrechte werden nach dem Minimalprinzip (Least Privilege) vergeben.
- Neue Berechtigungen erfordern die schriftliche Genehmigung der Führungskraft.
- Bei Stellenwechsel oder Ausscheiden werden Berechtigungen sofort angepasst bzw. entzogen.
- Quartalsweise findet eine Überprüfung aller Berechtigungen statt (Access Review).

## 6. Privilegierte Konten

Administrationskonten werden ausschließlich für administrative Tätigkeiten verwendet. Die Nutzung privilegierter Konten wird protokolliert und regelmäßig auditiert.`,
		tags:      []string{"iso27001", "nis2"},
		framework: strPtr("ISO 27001"),
	},
	{
		category: "policy",
		name:     "Datenschutzrichtlinie für Mitarbeiter",
		description: "Datenschutzinformation für Mitarbeiter gemäß Art. 13 DSGVO. Erläutert, welche personenbezogenen " +
			"Daten im Beschäftigungsverhältnis verarbeitet werden.",
		content: `# Datenschutzrichtlinie für Mitarbeiter (Art. 13 DSGVO)

## 1. Verantwortliche Stelle

Verantwortlich für die Verarbeitung Ihrer personenbezogenen Daten ist die Organisation (nachfolgend „Arbeitgeber"). Bei datenschutzrechtlichen Fragen wenden Sie sich bitte an den Datenschutzbeauftragten (DSB).

## 2. Verarbeitete Datenkategorien

Im Rahmen des Beschäftigungsverhältnisses verarbeitet der Arbeitgeber folgende Kategorien personenbezogener Daten:

- **Stammdaten**: Name, Anschrift, Geburtsdatum, Nationalität
- **Vertragsdaten**: Stellenbezeichnung, Eintrittsdatum, Vergütung, Arbeitszeitmodell
- **Leistungsdaten**: Beurteilungen, Schulungsnachweise, Arbeitsergebnisse
- **IT-Nutzungsdaten**: Protokolle der System- und Netzwerknutzung im Rahmen der gesetzlichen Vorgaben
- **Gesundheitsdaten** (nur soweit arbeitsrechtlich erforderlich): Arbeitsunfähigkeitsbescheinigungen

## 3. Rechtsgrundlagen der Verarbeitung

Die Verarbeitung erfolgt auf Basis von:
- Art. 6 Abs. 1 lit. b DSGVO (Vertragserfüllung) für die Durchführung des Arbeitsverhältnisses
- Art. 6 Abs. 1 lit. c DSGVO (rechtliche Verpflichtung) für gesetzlich vorgeschriebene Meldungen
- Art. 6 Abs. 1 lit. f DSGVO (berechtigtes Interesse) für IT-Sicherheitsmaßnahmen
- § 26 BDSG für beschäftigungsbezogene Zwecke

## 4. Empfänger der Daten

Personenbezogene Daten werden nur an folgende Empfänger weitergegeben:
- Behörden (z. B. Finanzamt, Sozialversicherungsträger) im gesetzlich vorgeschriebenen Umfang
- Auftragsverarbeiter (z. B. Lohnabrechnungsdienstleister) auf Basis eines AVV gemäß Art. 28 DSGVO
- Keine Übermittlung in Drittländer ohne angemessenes Schutzniveau

## 5. Speicherdauer

Daten werden gelöscht, sobald der Zweck entfällt und keine gesetzlichen Aufbewahrungsfristen entgegenstehen. In der Regel erfolgt die Löschung 6 Monate nach Beendigung des Beschäftigungsverhältnisses, soweit keine längeren steuer- oder sozialrechtlichen Aufbewahrungspflichten gelten.

## 6. Betroffenenrechte

Sie haben das Recht auf Auskunft (Art. 15), Berichtigung (Art. 16), Löschung (Art. 17), Einschränkung (Art. 18) und Datenübertragbarkeit (Art. 20 DSGVO). Bei Beschwerden können Sie sich an die zuständige Datenschutzaufsichtsbehörde wenden.`,
		tags:      []string{"dsgvo"},
		framework: nil,
	},
	{
		category: "policy",
		name:     "Richtlinie zur Nutzung von IT-Geräten und IT-Ressourcen",
		description: "Acceptable-Use-Policy für IT-Geräte, Internet und E-Mail. Regelt erlaubte und verbotene " +
			"Nutzung von Unternehmensressourcen.",
		content: `# Richtlinie zur Nutzung von IT-Geräten und IT-Ressourcen (Acceptable Use Policy)

## 1. Geltungsbereich

Diese Richtlinie gilt für alle Mitarbeiter, externen Dienstleister und sonstigen Personen, die IT-Ressourcen der Organisation nutzen. Sie umfasst Computer, mobile Endgeräte, Netzwerkinfrastruktur, E-Mail-Systeme, Cloud-Dienste und Internetzugang.

## 2. Grundsatz der dienstlichen Nutzung

IT-Ressourcen der Organisation dienen primär dienstlichen Zwecken. Eine eingeschränkte private Nutzung ist toleriert, sofern sie die berufliche Produktivität nicht beeinträchtigt, keine zusätzlichen Kosten verursacht und nicht gegen diese Richtlinie verstößt.

## 3. Erlaubte Nutzung

- Dienstliche Kommunikation per E-Mail und genehmigten Messenger-Diensten
- Nutzung von dienstlich genehmigten Cloud-Diensten und SaaS-Anwendungen
- Informationsrecherche für dienstliche Zwecke
- Private Nutzung in geringem Umfang außerhalb der Kernarbeitszeit

## 4. Verbotene Aktivitäten

Folgende Aktivitäten sind ausdrücklich untersagt:

- Download, Speicherung oder Verbreitung von urheberrechtlich geschütztem Material ohne Lizenz
- Zugriff auf pornografische, extremistische, illegale oder ethisch bedenkliche Inhalte
- Installation nicht genehmigter Software (Ausnahmen mit ISB-Freigabe)
- Umgehung von Sicherheitsmaßnahmen (Firewall, Proxy, DLP-Systeme)
- Nutzung für kommerzielle Eigeninteressen oder Nebentätigkeiten
- Versenden von Massenmails oder Spam
- Weitergabe vertraulicher Unternehmensdaten an unbefugte Dritte

## 5. E-Mail und Kommunikation

- E-Mails repräsentieren die Organisation und sind professionell zu verfassen.
- Verdächtige E-Mails und Phishing-Versuche sind dem IT-Helpdesk zu melden.
- Vertrauliche Daten dürfen nicht unverschlüsselt per E-Mail übertragen werden.
- Automatische Weiterleitungen auf externe Postfächer sind nicht gestattet.

## 6. Überwachung und Kontrolle

Die Organisation behält sich vor, die Nutzung von IT-Ressourcen im gesetzlich zulässigen und mit dem Betriebsrat abgestimmten Rahmen zu überwachen. Protokolldaten werden nur anlassbezogen ausgewertet.

## 7. Verlust und Diebstahl

Verlust oder Diebstahl von IT-Geräten ist unverzüglich dem IT-Helpdesk und der Führungskraft zu melden, damit Fernlöschung und Zugangssperrung eingeleitet werden können.`,
		tags:      []string{"iso27001"},
		framework: strPtr("ISO 27001"),
	},
	{
		category: "policy",
		name:     "Incident-Response-Richtlinie",
		description: "Richtlinie zur Erkennung, Meldung und strukturierten Behandlung von IT-Sicherheitsvorfällen. " +
			"NIS2-konform mit 72-Stunden-Meldefrist.",
		content: `# Incident-Response-Richtlinie

## 1. Zweck und Anwendungsbereich

Diese Richtlinie beschreibt das Vorgehen bei IT-Sicherheitsvorfällen, um Schäden zu minimieren, betroffene Systeme schnell wiederherzustellen und gesetzlichen Meldepflichten nachzukommen. Sie gilt für alle Mitarbeiter und IT-Systeme der Organisation.

## 2. Definition eines Sicherheitsvorfalls

Ein Sicherheitsvorfall ist jedes Ereignis, das die Vertraulichkeit, Integrität oder Verfügbarkeit von Informationen oder IT-Systemen der Organisation gefährdet oder gefährden könnte. Beispiele:

- Malware-Infektion oder Ransomware-Angriff
- Unbefugter Zugriff auf Systeme oder Daten
- Datenpanne (versehentliche Offenlegung personenbezogener Daten)
- DDoS-Angriff oder Denial-of-Service-Ereignis
- Verlust oder Diebstahl von Geräten mit Unternehmensdaten

## 3. Meldepflichten für Mitarbeiter

Alle Mitarbeiter sind verpflichtet, erkannte oder vermutete Sicherheitsvorfälle **unverzüglich** zu melden:

- **Intern**: security@[organisation].de oder Ticket im IT-Helpdesk
- **Telefonisch**: [Sicherheits-Hotline eintragen]
- **Außerhalb der Bürozeiten**: Erreichbarkeit des ISB sicherstellen

## 4. Reaktionsphasen (PICERL)

1. **Vorbereitung (Prepare)**: Notfallpläne, Kontaktlisten, Werkzeuge bereithalten.
2. **Identifikation (Identify)**: Vorfall erkennen, klassifizieren und Schweregrad bestimmen.
3. **Eindämmung (Contain)**: Betroffene Systeme isolieren, Ausbreitung verhindern.
4. **Beseitigung (Eradicate)**: Schadcode entfernen, Schwachstellen schließen.
5. **Wiederherstellung (Recover)**: Systeme aus sauberen Backups wiederherstellen, Funktionsfähigkeit prüfen.
6. **Nachbereitung (Lessons Learned)**: Ursachenanalyse, Dokumentation, Maßnahmen ableiten.

## 5. Gesetzliche Meldepflichten

### DSGVO Art. 33 (Datenpannen)
Bei Datenpannen mit Risiko für betroffene Personen: Meldung an die zuständige Datenschutzaufsichtsbehörde **innerhalb von 72 Stunden** nach Bekanntwerden.

### NIS2-Richtlinie
Betreiber wesentlicher und wichtiger Einrichtungen melden erhebliche Vorfälle:
- **Frühwarnung** (Stufe 1): innerhalb von 24 Stunden
- **Erstmeldung** (Stufe 2): innerhalb von 72 Stunden
- **Abschlussbericht**: innerhalb von 1 Monat

## 6. Dokumentation

Alle Vorfälle werden im Vorfallsregister (Incident Register) dokumentiert, unabhängig von ihrer Schwere. Die Dokumentation umfasst: Zeitstempel, Beschreibung, betroffene Systeme, ergriffene Maßnahmen und Lessons Learned.`,
		tags:      []string{"nis2", "iso27001"},
		framework: strPtr("NIS2"),
	},

	// --- DPIAs ---
	{
		category: "dpia",
		name:     "DSFA-Vorlage: HR-System",
		description: "Standardvorlage für eine Datenschutz-Folgenabschätzung (DSFA) gemäß Art. 35 DSGVO für " +
			"HR-Systeme, Mitarbeiterdaten und Bewerbermanagement.",
		content: `# Datenschutz-Folgenabschätzung (DSFA) — HR-System

**Gemäß Art. 35 DSGVO**

## 1. Beschreibung des Verarbeitungsvorgangs

### 1.1 Zweck der Verarbeitung
Das HR-System dient der Verwaltung von Mitarbeiterstammdaten, Bewerberdaten, Gehaltsabrechnungen, Abwesenheitsmanagement und Leistungsbeurteilungen.

### 1.2 Art der verarbeiteten Daten
- Identifikationsdaten (Name, Geburtsdatum, Anschrift)
- Vertragsdaten (Gehalt, Arbeitszeitmodell, Beurteilungen)
- Gesundheitsdaten (Krankheitstage, Schwerbehinderung soweit relevant)
- Bankverbindungsdaten für die Gehaltsauszahlung
- Bewerbungsunterlagen (Lebenslauf, Zeugnisse)

### 1.3 Betroffene Personengruppen
Aktive Mitarbeiter, ehemalige Mitarbeiter (im Rahmen gesetzlicher Aufbewahrungsfristen), Bewerber.

### 1.4 Empfänger
Lohnbuchhaltungsdienstleister (Auftragsverarbeiter), Sozialversicherungsträger, Finanzamt, interne HR-Mitarbeiter.

## 2. Notwendigkeit und Verhältnismäßigkeit

Die Verarbeitung ist erforderlich für die Erfüllung des Arbeitsvertrags (Art. 6 Abs. 1 lit. b DSGVO), gesetzlicher Verpflichtungen (lit. c) sowie berechtigter Interessen (lit. f). Gesundheitsdaten werden nur auf Basis von § 26 Abs. 3 BDSG i. V. m. Art. 9 Abs. 2 lit. b DSGVO verarbeitet.

## 3. Risikoidentifikation und -bewertung

| Risiko | Eintrittswahrscheinlichkeit | Schwere | Risikostufe |
|---|---|---|---|
| Unbefugter Zugriff durch externe Angreifer | Mittel | Hoch | Hoch |
| Missbrauch durch interne Mitarbeiter | Niedrig | Hoch | Mittel |
| Datenverlust durch technische Fehler | Niedrig | Mittel | Niedrig |
| Weitergabe an unberechtigte Dritte | Niedrig | Hoch | Mittel |
| Identitätsdiebstahl bei Datenpanne | Niedrig | Hoch | Mittel |

## 4. Geplante Abhilfemaßnahmen (TOMs)

- Zugriffsberechtigungen nach dem Minimalprinzip (Role-Based Access Control)
- Verschlüsselung aller Datenbankverbindungen (TLS 1.2+) und ruhender Daten (AES-256)
- Multi-Faktor-Authentifizierung für den HR-System-Zugang
- Protokollierung aller Zugriffe auf sensitive Datenkategorien
- Pseudonymisierung in Testumgebungen
- Regelmäßige Backups mit getesteter Wiederherstellung
- Auftragsverarbeitungsvertrag (AVV) mit allen Dienstleistern
- Schulung der HR-Mitarbeiter zu Datenschutzpflichten

## 5. Konsultation des Datenschutzbeauftragten

☐ DSB wurde konsultiert am: _______________
☐ DSB-Stellungnahme liegt vor

## 6. Ergebnis der DSFA

Nach Umsetzung der beschriebenen Maßnahmen ist das Restrisiko als **akzeptabel** einzustufen. Eine vorherige Konsultation der Aufsichtsbehörde gemäß Art. 36 DSGVO ist **nicht erforderlich**.

_Verantwortlicher: _______________ | Datum: _______________ | DSB: _______________`,
		tags:      []string{"dsgvo", "art35"},
		framework: nil,
	},
	{
		category: "dpia",
		name:     "DSFA-Vorlage: Kundendatenbank",
		description: "Standardvorlage für eine Datenschutz-Folgenabschätzung gemäß Art. 35 DSGVO für " +
			"CRM-Systeme und Kundendatenbanken.",
		content: `# Datenschutz-Folgenabschätzung (DSFA) — Kundendatenbank / CRM

**Gemäß Art. 35 DSGVO**

## 1. Beschreibung des Verarbeitungsvorgangs

### 1.1 Zweck der Verarbeitung
Das CRM-System dient der Verwaltung von Kundenstammdaten, Kommunikationshistorie, Vertragsabwicklung und Marketing-Aktivitäten.

### 1.2 Art der verarbeiteten Daten
- Kontaktdaten (Name, E-Mail, Telefon, Anschrift)
- Vertragsdaten (Bestellhistorie, Rechnungsstellung)
- Kommunikationshistorie (E-Mails, Gesprächsnotizen)
- Zahlungsdaten (soweit relevant, über zertifizierten Payment-Provider)
- Präferenzdaten für Marketing (mit Einwilligung)

### 1.3 Betroffene Personengruppen
Bestands- und Neukunden, Interessenten, ehemalige Kunden (im Rahmen gesetzlicher Aufbewahrungsfristen).

### 1.4 Empfänger
Interne Vertriebsabteilung, Marketing, Buchhaltung; externe Auftragsverarbeiter (z. B. E-Mail-Marketing-Tool, nur nach AVV und mit Einwilligung).

## 2. Notwendigkeit und Verhältnismäßigkeit

Die Verarbeitung ist überwiegend auf Art. 6 Abs. 1 lit. b DSGVO (Vertragserfüllung) und lit. f (berechtigtes Interesse an Kundenpflege) gestützt. Marketing-Aktivitäten per E-Mail erfordern eine Einwilligung gemäß Art. 6 Abs. 1 lit. a DSGVO (Double-Opt-In).

Datenminimierung: Es werden nur die für den Verarbeitungszweck notwendigen Daten erhoben. Präferenzdaten werden nur nach expliziter Einwilligung gespeichert.

## 3. Risikoidentifikation und -bewertung

| Risiko | Eintrittswahrscheinlichkeit | Schwere | Risikostufe |
|---|---|---|---|
| Unbefugter Zugriff durch externe Angreifer | Mittel | Hoch | Hoch |
| SQL-Injection oder andere Angriffe auf die Applikation | Niedrig | Hoch | Mittel |
| Missbrauch durch interne Mitarbeiter | Niedrig | Mittel | Niedrig |
| Unberechtigte Weitergabe an Dritte | Niedrig | Hoch | Mittel |
| Datenverlust (ohne Backup) | Niedrig | Hoch | Mittel |

## 4. Geplante Abhilfemaßnahmen (TOMs)

- Zugriffscontrolling nach Abteilung und Rolle (RBAC)
- Verschlüsselung der Datenbankverbindungen (TLS) und gespeicherter Daten
- Web Application Firewall (WAF) zum Schutz vor Injection-Angriffen
- Protokollierung von Zugriffen und Änderungen (Audit Log, mindestens 90 Tage)
- Getestetes Backup-Konzept (täglich, 30 Tage Aufbewahrung)
- Datenlöschkonzept: automatisierte Löschung nach Ablauf der Aufbewahrungsfristen
- Einwilligungsmanagement mit Opt-In/Opt-Out und nachvollziehbarer Dokumentation
- AVV mit allen Auftragsverarbeitern (z. B. E-Mail-Marketing-Tool)

## 5. Konsultation des Datenschutzbeauftragten

☐ DSB wurde konsultiert am: _______________
☐ DSB-Stellungnahme liegt vor

## 6. Ergebnis der DSFA

Nach Umsetzung der beschriebenen Maßnahmen ist das Restrisiko als **akzeptabel** einzustufen.

_Verantwortlicher: _______________ | Datum: _______________ | DSB: _______________`,
		tags:      []string{"dsgvo", "art35"},
		framework: nil,
	},

	// --- AVVs ---
	{
		category: "avv",
		name:     "AVV-Standardvorlage (Art. 28 DSGVO)",
		description: "Standardvorlage für einen Auftragsverarbeitungsvertrag gemäß Art. 28 DSGVO. Geeignet für " +
			"SaaS-Dienstleister, Cloud-Anbieter und sonstige Auftragsverarbeiter.",
		content: `# Auftragsverarbeitungsvertrag (AVV)
**gemäß Art. 28 Datenschutz-Grundverordnung (DSGVO)**

---

**zwischen**

[Name der Organisation] (nachfolgend „**Verantwortlicher**")
[Anschrift]

**und**

[Name des Dienstleisters] (nachfolgend „**Auftragsverarbeiter**")
[Anschrift]

---

## § 1 Gegenstand und Dauer der Beauftragung

(1) Der Auftragsverarbeiter erbringt für den Verantwortlichen folgende Leistungen: [Beschreibung der Leistung].

(2) Die Beauftragung beginnt mit Unterzeichnung dieses Vertrags und endet mit der Kündigung des zugrundeliegenden Hauptvertrags.

## § 2 Weisungsgebundenheit

(1) Der Auftragsverarbeiter verarbeitet personenbezogene Daten ausschließlich auf dokumentierte Weisung des Verantwortlichen.

(2) Ist der Auftragsverarbeiter der Ansicht, dass eine Weisung gegen die DSGVO oder andere Datenschutzvorschriften verstößt, informiert er den Verantwortlichen unverzüglich.

## § 3 Art und Zweck der Verarbeitung, Art der Daten, Kategorien betroffener Personen

| Merkmal | Beschreibung |
|---|---|
| Verarbeitungszweck | [z. B. Hosting, CRM, E-Mail-Versand] |
| Art der Daten | [z. B. Kontaktdaten, Nutzungsdaten] |
| Kategorien betroffener Personen | [z. B. Kunden, Mitarbeiter] |

Besondere Kategorien (Art. 9 DSGVO) werden **nicht** verarbeitet, sofern nicht ausdrücklich gesondert vereinbart.

## § 4 Technische und Organisatorische Maßnahmen (TOMs)

Der Auftragsverarbeiter trifft die in Anlage 1 beschriebenen technischen und organisatorischen Maßnahmen gemäß Art. 32 DSGVO. Diese umfassen mindestens:

- Verschlüsselung (Transport und Speicherung)
- Zutrittskontrolle, Zugangskontrolle, Zugriffskontrolle
- Weitergabekontrolle, Eingabekontrolle
- Verfügbarkeitskontrolle (Backup, Redundanz)
- Verfahren zur regelmäßigen Überprüfung, Bewertung und Evaluierung

## § 5 Unterauftragsverhältnisse

(1) Die Inanspruchnahme von Unterauftragsverarbeitern bedarf der vorherigen schriftlichen Genehmigung des Verantwortlichen. Eine Generalgenehmigung für bestimmte Unterauftragsverarbeiter ist in Anlage 2 festgehalten.

(2) Der Auftragsverarbeiter verpflichtet Unterauftragsverarbeiter vertraglich zu denselben Datenschutzpflichten.

## § 6 Unterstützungspflichten

Der Auftragsverarbeiter unterstützt den Verantwortlichen bei der Erfüllung von Betroffenenrechten (Art. 15–22 DSGVO) sowie bei der Einhaltung der in Art. 32–36 DSGVO genannten Pflichten.

## § 7 Löschung und Rückgabe von Daten

Nach Beendigung der Verarbeitung löscht oder gibt der Auftragsverarbeiter alle personenbezogenen Daten zurück, sofern keine gesetzliche Aufbewahrungspflicht entgegensteht.

## § 8 Nachweispflichten und Prüfungsrecht

Der Auftragsverarbeiter stellt dem Verantwortlichen alle erforderlichen Informationen zur Verfügung und ermöglicht Audits durch den Verantwortlichen oder einen beauftragten Prüfer.

## § 9 Meldepflichten bei Datenpannen

Der Auftragsverarbeiter meldet Datenpannen unverzüglich, spätestens innerhalb von 24 Stunden nach Bekanntwerden, an den Verantwortlichen.

---

_Ort, Datum: ________________

_Verantwortlicher: _______________ | Auftragsverarbeiter: _______________`,
		tags:      []string{"dsgvo", "art28"},
		framework: nil,
	},
}

// SeedPolicyTemplates inserts the built-in compliance templates into the database
// if the ck_policy_templates table is empty. It is idempotent and safe to call on
// every startup.
func SeedPolicyTemplates(ctx context.Context, db *pgxpool.Pool) error {
	var count int
	// orgid-lint: global — ck_policy_templates is a shared system catalogue, not per-org
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM ck_policy_templates`).Scan(&count); err != nil {
		return fmt.Errorf("seed policy templates: count check: %w", err)
	}
	if count > 0 {
		return nil // already seeded
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("seed policy templates: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, t := range builtinDBTemplates {
		_, err := tx.Exec(ctx, `
			INSERT INTO ck_policy_templates (category, name, description, content, tags, framework)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING
		`, t.category, t.name, t.description, t.content, t.tags, t.framework)
		if err != nil {
			return fmt.Errorf("seed policy templates: insert %q: %w", t.name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("seed policy templates: commit: %w", err)
	}
	return nil
}
