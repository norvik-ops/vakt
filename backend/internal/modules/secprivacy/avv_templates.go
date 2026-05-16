package secprivacy

// AVVTemplate is a pre-built Auftragsverarbeitungsvertrag template with German legal content.
type AVVTemplate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`     // Markdown with {{placeholders}}
	Variables   []string `json:"variables"` // placeholder names without {{ }}
}

// BuiltinAVVTemplates returns the built-in German AVV templates.
func BuiltinAVVTemplates() []AVVTemplate {
	return []AVVTemplate{
		{
			ID:          "avv-standard",
			Title:       "AVV Standard (DSGVO Art. 28)",
			Description: "Allgemeiner Auftragsverarbeitungsvertrag für typische Dienstleister gemäß Art. 28 DSGVO.",
			Variables:   []string{"auftraggeber", "auftragnehmer", "datum", "zweck"},
			Body: `# Auftragsverarbeitungsvertrag
## gemäß Art. 28 Datenschutz-Grundverordnung (DSGVO)

**Stand:** {{datum}}

---

## Präambel

Zwischen

**{{auftraggeber}}** (nachfolgend „Auftraggeber" oder „Verantwortlicher")

und

**{{auftragnehmer}}** (nachfolgend „Auftragnehmer" oder „Auftragsverarbeiter")

wird nachfolgender Auftragsverarbeitungsvertrag geschlossen.

---

## § 1 Gegenstand und Dauer der Beauftragung

(1) Der Auftragnehmer verarbeitet personenbezogene Daten im Auftrag des Auftraggebers. Gegenstand und Zweck der Beauftragung ist: **{{zweck}}**.

(2) Die Beauftragung beginnt mit Unterzeichnung dieses Vertrages und gilt auf unbestimmte Zeit, sofern keine abweichende Vereinbarung getroffen wurde.

(3) Die Verarbeitung erfolgt ausschließlich im Gebiet der Europäischen Union bzw. des Europäischen Wirtschaftsraums, sofern nicht ausdrücklich anders vereinbart.

---

## § 2 Art und Zweck der Verarbeitung

(1) Die Verarbeitung umfasst insbesondere folgende Tätigkeiten: Erhebung, Speicherung, Übermittlung, Nutzung und Löschung personenbezogener Daten im Rahmen der vertraglich vereinbarten Leistungserbringung.

(2) Art der betroffenen Daten: Stammdaten, Kontaktdaten sowie weitere im Leistungsvertrag spezifizierte Datenkategorien.

(3) Kreis der Betroffenen: Kunden, Interessenten, Mitarbeiter und sonstige Personen, deren Daten im Rahmen der Leistungserbringung verarbeitet werden.

---

## § 3 Pflichten des Auftragnehmers

(1) Der Auftragnehmer verarbeitet personenbezogene Daten ausschließlich auf dokumentierte Weisung des Auftraggebers, es sei denn, er ist durch das Recht der Union oder der Mitgliedstaaten, dem er unterliegt, zur Verarbeitung verpflichtet.

(2) Der Auftragnehmer gewährleistet, dass die zur Verarbeitung befugten Personen zur Vertraulichkeit verpflichtet worden sind oder einer angemessenen gesetzlichen Verschwiegenheitspflicht unterliegen.

(3) Der Auftragnehmer ergreift alle nach Art. 32 DSGVO erforderlichen technischen und organisatorischen Maßnahmen zum Schutz der verarbeiteten Daten.

(4) Der Auftragnehmer unterstützt den Auftraggeber bei der Einhaltung der in den Art. 32 bis 36 DSGVO genannten Pflichten.

(5) Der Auftragnehmer löscht nach Abschluss der Erbringung der Verarbeitungsleistungen alle personenbezogenen Daten und bestehende Kopien, soweit nicht das Unionsrecht oder das Recht der Mitgliedstaaten eine Speicherung erfordert.

---

## § 4 Subunternehmer (weitere Auftragsverarbeiter)

(1) Der Auftragnehmer darf weitere Auftragsverarbeiter (Subunternehmer) nur mit vorheriger gesonderter oder allgemeiner schriftlicher Genehmigung des Auftraggebers hinzuziehen.

(2) Bereits zum Zeitpunkt des Vertragsschlusses eingesetzte Subunternehmer gelten als genehmigt und sind in Anlage 1 aufgeführt.

(3) Der Auftragnehmer legt dem Auftraggeber auf Verlangen jederzeit die Verzeichnisse über Subunternehmer vor.

---

## § 5 Kontrollrechte des Auftraggebers

(1) Der Auftraggeber ist berechtigt, die Einhaltung dieses Vertrages und der datenschutzrechtlichen Anforderungen durch den Auftragnehmer zu kontrollieren. Der Auftragnehmer stellt dem Auftraggeber alle erforderlichen Informationen zum Nachweis der Einhaltung der in Art. 28 DSGVO niedergelegten Pflichten zur Verfügung.

(2) Der Auftragnehmer ermöglicht dem Auftraggeber oder einem von ihm beauftragten Prüfer Audits, einschließlich Inspektionen, und trägt zu diesen bei.

---

## § 6 Meldepflichten bei Datenpannen

Der Auftragnehmer informiert den Auftraggeber unverzüglich — in der Regel innerhalb von 24 Stunden — über Verletzungen des Schutzes personenbezogener Daten (Art. 33 DSGVO), damit der Auftraggeber seiner Meldepflicht gegenüber den Aufsichtsbehörden fristgerecht nachkommen kann.

---

## § 7 Schlussbestimmungen

(1) Dieser Vertrag unterliegt dem Recht der Bundesrepublik Deutschland.

(2) Änderungen und Ergänzungen dieses Vertrages bedürfen der Schriftform.

(3) Sollten einzelne Bestimmungen dieses Vertrages ganz oder teilweise unwirksam sein oder werden, so berührt dies die Wirksamkeit der übrigen Bestimmungen nicht.

---

_{{auftraggeber}}_

_{{auftragnehmer}}_
`,
		},
		{
			ID:          "avv-cloud",
			Title:       "AVV Cloud-Dienstleister",
			Description: "Auftragsverarbeitungsvertrag für Cloud-Dienste (IaaS, PaaS, SaaS) gemäß Art. 28 DSGVO.",
			Variables:   []string{"auftraggeber", "auftragnehmer", "datum", "zweck"},
			Body: `# Auftragsverarbeitungsvertrag — Cloud-Dienste
## gemäß Art. 28 Datenschutz-Grundverordnung (DSGVO)

**Stand:** {{datum}}

---

## Präambel

Zwischen

**{{auftraggeber}}** (nachfolgend „Auftraggeber")

und

**{{auftragnehmer}}** (nachfolgend „Cloud-Anbieter" oder „Auftragsverarbeiter")

wird nachfolgender Auftragsverarbeitungsvertrag für Cloud-Dienstleistungen geschlossen.

---

## § 1 Gegenstand der Beauftragung

(1) Der Cloud-Anbieter stellt dem Auftraggeber Cloud-Dienste zur Verfügung. Im Rahmen dieser Leistungserbringung verarbeitet der Cloud-Anbieter personenbezogene Daten im Auftrag des Auftraggebers. Zweck der Verarbeitung: **{{zweck}}**.

(2) Die Verarbeitung der Daten erfolgt ausschließlich auf Servern innerhalb des Europäischen Wirtschaftsraums (EWR), sofern nicht vertraglich ausdrücklich anders geregelt und die Voraussetzungen der Art. 44 ff. DSGVO erfüllt sind.

---

## § 2 Technische und organisatorische Maßnahmen (TOMs)

(1) Der Cloud-Anbieter implementiert und unterhält angemessene technische und organisatorische Sicherheitsmaßnahmen gemäß Art. 32 DSGVO, die mindestens Folgendes umfassen:

- **Zugangskontrolle:** Verschlüsselte Authentifizierung, Multi-Faktor-Authentifizierung für Administratorzugänge
- **Zugriffskontrolle:** Rollenbasierte Rechteverwaltung, Prinzip der minimalen Rechtevergabe
- **Weitergabekontrolle:** Verschlüsselte Datenübertragung (TLS 1.2 oder höher), VPN-Verbindungen
- **Eingabekontrolle:** Protokollierung aller Zugriffsaktivitäten, unveränderliche Audit-Logs
- **Verfügbarkeitskontrolle:** Redundante Systeme, automatisierte Backups, Disaster-Recovery-Konzept
- **Trennungsgebot:** Logische Mandantentrennung, Datenisolierung zwischen Kunden

(2) Der Cloud-Anbieter ist im Besitz einschlägiger Zertifizierungen (z. B. ISO 27001, SOC 2 Type II) und stellt dem Auftraggeber auf Verlangen aktuelle Nachweise zur Verfügung.

---

## § 3 Subunternehmer und Rechenzentren

(1) Der Cloud-Anbieter benennt dem Auftraggeber alle eingesetzten Subunternehmer und Rechenzentren. Der Auftraggeber erteilt eine allgemeine Genehmigung zur Nutzung von Subunternehmern, sofern diese in der Anlage zu diesem Vertrag aufgeführt sind.

(2) Der Cloud-Anbieter verpflichtet alle Subunternehmer vertraglich auf denselben Datenschutzstandard wie in diesem Vertrag vereinbart.

(3) Änderungen am Subunternehmer-Kreis werden dem Auftraggeber mit einer Vorlaufzeit von mindestens 30 Tagen angekündigt. Der Auftraggeber hat das Recht, Änderungen zu widersprechen.

---

## § 4 Datenrückgabe und -löschung

(1) Der Auftraggeber kann seine Daten jederzeit in einem gängigen, maschinenlesbaren Format exportieren.

(2) Nach Vertragsende löscht der Cloud-Anbieter alle personenbezogenen Daten des Auftraggebers, sofern keine gesetzliche Aufbewahrungspflicht besteht. Die Löschung wird auf Verlangen schriftlich bestätigt.

---

## § 5 Datenpannen und Sicherheitsvorfälle

Der Cloud-Anbieter meldet Sicherheitsvorfälle, die personenbezogene Daten des Auftraggebers betreffen, unverzüglich und spätestens innerhalb von 24 Stunden an den Auftraggeber. Die Meldung enthält mindestens: Art des Vorfalls, betroffene Datenkategorien und -mengen, wahrscheinliche Folgen sowie ergriffene Maßnahmen.

---

## § 6 Schlussbestimmungen

Dieser Vertrag ergänzt den Hauptvertrag zwischen den Parteien. Bei Widersprüchen zwischen diesem Vertrag und dem Hauptvertrag geht dieser AVV in datenschutzrechtlichen Fragen vor.

---

_{{auftraggeber}}_

_{{auftragnehmer}}_
`,
		},
		{
			ID:          "avv-subprocessor",
			Title:       "Unterauftragnehmer-Vereinbarung",
			Description: "Vertrag für den Einsatz von Unterauftragsverarbeitern (Sub-Processors) durch den Auftragnehmer.",
			Variables:   []string{"auftraggeber", "auftragnehmer", "datum", "zweck"},
			Body: `# Unterauftragnehmer-Vereinbarung
## gemäß Art. 28 Abs. 4 Datenschutz-Grundverordnung (DSGVO)

**Stand:** {{datum}}

---

## Präambel

Zwischen

**{{auftraggeber}}** (nachfolgend „Hauptauftraggeber")

und

**{{auftragnehmer}}** (nachfolgend „Unterauftragnehmer")

wird nachfolgende Vereinbarung zur Unterbeauftragung im Rahmen der Auftragsverarbeitung geschlossen.

---

## § 1 Hintergrund und Gegenstand

(1) Der Hauptauftraggeber ist seinerseits als Auftragsverarbeiter für einen Verantwortlichen tätig und wurde durch den Verantwortlichen zur Unterbeauftragung ermächtigt.

(2) Der Unterauftragnehmer wird im Rahmen dieser Ermächtigung damit beauftragt, personenbezogene Daten im Unterauftrag zu verarbeiten. Zweck der Verarbeitung: **{{zweck}}**.

(3) Diese Vereinbarung regelt die datenschutzrechtlichen Pflichten des Unterauftragnehmers gemäß Art. 28 Abs. 4 DSGVO.

---

## § 2 Pflichten des Unterauftragnehmers

(1) Der Unterauftragnehmer unterliegt denselben datenschutzrechtlichen Verpflichtungen wie der Hauptauftraggeber gegenüber dem Verantwortlichen. Dies umfasst insbesondere:

- Verarbeitung ausschließlich auf dokumentierte Weisung des Hauptauftraggebers
- Vertraulichkeitsverpflichtung aller mit der Verarbeitung befassten Personen
- Implementierung angemessener technischer und organisatorischer Maßnahmen (Art. 32 DSGVO)
- Unterstützung bei der Erfüllung der Betroffenenrechte (Art. 12–22 DSGVO)
- Meldung von Datenpannen innerhalb von 24 Stunden

(2) Der Unterauftragnehmer darf ohne ausdrückliche schriftliche Genehmigung des Hauptauftraggebers keine weiteren Unterauftragsverarbeiter hinzuziehen.

---

## § 3 Verarbeitungsort und Datentransfers

(1) Die Verarbeitung erfolgt ausschließlich innerhalb der Europäischen Union oder des Europäischen Wirtschaftsraums.

(2) Jede Verarbeitung außerhalb des EWR bedarf der vorherigen schriftlichen Genehmigung des Hauptauftraggebers und setzt das Vorliegen geeigneter Garantien gemäß Art. 44 ff. DSGVO voraus (z. B. Standarddatenschutzklauseln der EU-Kommission).

---

## § 4 Nachweispflichten und Audits

(1) Der Unterauftragnehmer stellt dem Hauptauftraggeber alle Informationen zur Verfügung, die zum Nachweis der Einhaltung seiner Datenschutzpflichten erforderlich sind.

(2) Der Unterauftragnehmer gestattet dem Hauptauftraggeber oder einem beauftragten Dritten die Durchführung von Audits und Inspektionen und unterstützt diese aktiv.

(3) Aktuelle Zertifizierungen (z. B. ISO 27001, SOC 2) können als gleichwertiger Nachweis akzeptiert werden.

---

## § 5 Haftung

(1) Der Unterauftragnehmer haftet dem Hauptauftraggeber gegenüber für Schäden, die durch eine nicht DSGVO-konforme Verarbeitung entstehen, im Rahmen der gesetzlichen Vorschriften.

(2) Der Unterauftragnehmer stellt den Hauptauftraggeber von Ansprüchen Dritter frei, soweit diese auf einem Verstoß des Unterauftragnehmers gegen seine Pflichten aus diesem Vertrag beruhen.

---

## § 6 Laufzeit und Kündigung

(1) Diese Vereinbarung gilt für die Dauer der Hauptbeauftragung. Sie endet automatisch mit Beendigung des zugrundeliegenden Auftragsverarbeitungsvertrages.

(2) Nach Vertragsende löscht der Unterauftragnehmer alle verarbeiteten Daten und belegt dies auf Verlangen schriftlich.

---

_{{auftraggeber}}_

_{{auftragnehmer}}_
`,
		},
	}
}

// SCCModule describes one of the four EU Standard Contractual Clauses modules.
type SCCModule struct {
	ID          string `json:"id"` // module_1 .. module_4
	Title       string `json:"title"`
	Description string `json:"description"`
}

// BuiltinSCCModules returns the four EU Standard Contractual Clauses modules
// as adopted by the EU Commission on 4 June 2021 (Decision 2021/914/EU).
func BuiltinSCCModules() []SCCModule {
	return []SCCModule{
		{
			ID:    "module_1",
			Title: "Modul 1 — Verantwortlicher zu Verantwortlichem (C2C)",
			Description: "Dieses Modul gilt für die Übermittlung personenbezogener Daten zwischen zwei Verantwortlichen " +
				"(Controller-to-Controller). Beide Parteien legen unabhängig voneinander den Zweck und die Mittel " +
				"der Datenverarbeitung fest. Typische Anwendungsfälle: gemeinsame Kundendatenbanken zwischen " +
				"Konzernunternehmen, Datenweitergabe zwischen Kooperationspartnern.",
		},
		{
			ID:    "module_2",
			Title: "Modul 2 — Verantwortlicher zu Auftragsverarbeiter (C2P)",
			Description: "Dieses Modul regelt die Übermittlung personenbezogener Daten von einem Verantwortlichen " +
				"(Controller) an einen Auftragsverarbeiter (Processor) in einem Drittland. Dies ist das häufigste " +
				"Szenario bei Cloud-Diensten, Hosting-Anbietern und anderen IT-Dienstleistern außerhalb des EWR " +
				"(z. B. US-amerikanische SaaS-Anbieter ohne Angemessenheitsbeschluss).",
		},
		{
			ID:    "module_3",
			Title: "Modul 3 — Auftragsverarbeiter zu Auftragsverarbeiter (P2P)",
			Description: "Dieses Modul gilt für die Übermittlung personenbezogener Daten zwischen zwei " +
				"Auftragsverarbeitern (Processor-to-Processor), d. h. wenn ein Auftragsverarbeiter Daten an " +
				"einen Unterauftragsverarbeiter in einem Drittland weitergibt. Voraussetzung ist die Genehmigung " +
				"des ursprünglichen Verantwortlichen. Typisch bei Sub-Cloud-Diensten oder spezialisierten " +
				"Unterdienstleistern.",
		},
		{
			ID:    "module_4",
			Title: "Modul 4 — Auftragsverarbeiter zu Verantwortlichem (P2C)",
			Description: "Dieses Modul deckt die Rückübermittlung personenbezogener Daten von einem " +
				"Auftragsverarbeiter im Drittland zurück an den Verantwortlichen im EWR ab " +
				"(Processor-to-Controller). Es greift z. B. wenn ein in den USA ansässiger Dienstleister " +
				"verarbeitete Ergebnisse oder Berichte mit personenbezogenen Daten an den europäischen " +
				"Auftraggeber zurücksendet.",
		},
	}
}
