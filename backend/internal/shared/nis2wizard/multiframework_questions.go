package nis2wizard

// Sprint 28 / S28-4: Multi-Framework-Assessment — ~80 Fragen für kombiniertes
// NIS2 + ISO 27001 + DSGVO-TOM-Assessment mit Cross-Mapping.
//
// IDs sind stabil — nie umbenennen, sonst brechen historische Runs.
// Cross-Mapping: Felder in CrossFrameworks listen alle Standards, die eine
// Frage abdeckt (z.B. eine NIS2-Frage, die auch ISO27001 A.8 abdeckt).

const (
	FrameworkNIS2     = "nis2"
	FrameworkISO27001 = "iso27001"
	FrameworkDSGVOTOM = "dsgvo_tom"
)

// MultiFrameworkQuestion beschreibt eine Frage im kombinierten Assessment.
type MultiFrameworkQuestion struct {
	ID               string   `json:"id"`
	Framework        string   `json:"framework"`          // primäres Framework
	CrossFrameworks  []string `json:"cross_frameworks"`   // weitere abgedeckte Frameworks
	Area             string   `json:"area"`
	Text             string   `json:"text"`
	HelpText         string   `json:"help_text"`
	Weight           float64  `json:"weight"`
	Ref              string   `json:"ref"`
}

// MultiFrameworkAreas enthält alle Area-Bezeichner pro Framework.
var MultiFrameworkAreas = map[string][]string{
	FrameworkNIS2: {
		"governance", "risk_management", "incident_response",
		"business_continuity", "supply_chain", "crypto",
		"access_control", "asset_management",
	},
	FrameworkISO27001: {
		"a5_org_controls", "a6_people", "a8_asset_mgmt", "a9_access_control",
	},
	FrameworkDSGVOTOM: {
		"datenschutz_grundsaetze", "technische_schutzmassnahmen", "auftragsverarbeitung",
	},
}

// MultiFrameworkAreaTitles liefert lesbare Titel für alle Bereiche.
var MultiFrameworkAreaTitles = map[string]string{
	// NIS2
	"governance":           "Governance & Verantwortlichkeit",
	"risk_management":      "Risikomanagement",
	"incident_response":    "Incident-Response",
	"business_continuity":  "Business Continuity",
	"supply_chain":         "Lieferketten-Sicherheit",
	"crypto":               "Kryptographie",
	"access_control":       "Zugriffskontrolle",
	"asset_management":     "Asset- & Vulnerability-Mgmt",
	// ISO 27001
	"a5_org_controls":      "ISO 27001 A.5 — Organisatorische Maßnahmen",
	"a6_people":            "ISO 27001 A.6 — Personalbezogene Maßnahmen",
	"a8_asset_mgmt":        "ISO 27001 A.8 — Asset-Management",
	"a9_access_control":    "ISO 27001 A.9 — Zugriffskontrolle",
	// DSGVO-TOM
	"datenschutz_grundsaetze":       "DSGVO — Datenschutz-Grundsätze (Art. 5)",
	"technische_schutzmassnahmen":   "DSGVO — Technische Schutzmaßnahmen (Art. 32)",
	"auftragsverarbeitung":          "DSGVO — Auftragsverarbeitung (Art. 28)",
}

// MultiFrameworkQuestions ist die kombinierte ~80-Fragen-Liste.
// Reihenfolge: NIS2 → ISO 27001 → DSGVO-TOM.
// Stable IDs — niemals umbenennen.
var MultiFrameworkQuestions = []MultiFrameworkQuestion{

	// ── NIS2: Governance (4) ────────────────────────────────────────────────
	{
		ID: "mf.nis2.gov.policy", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "governance",
		Text: "Existiert eine schriftliche Informationssicherheits-Leitlinie?",
		HelpText: "Vom Top-Management freigegeben, mindestens einmal jährlich überprüft und kommuniziert.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(a) / ISO 27001 A.5.1",
	},
	{
		ID: "mf.nis2.gov.responsibility", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "governance",
		Text: "Ist ein CISO oder Sicherheitsverantwortlicher mit klaren Befugnissen benannt?",
		HelpText: "Dokumentierte Verantwortlichkeiten + Berichtsweg zur Geschäftsführung.",
		Weight: 2, Ref: "NIS2 Art. 20(1) / ISO 27001 A.5.2",
	},
	{
		ID: "mf.nis2.gov.training", Framework: FrameworkNIS2,
		Area: "governance",
		Text: "Erhält die Geschäftsleitung jährliche NIS2-spezifische Schulungen?",
		HelpText: "Pflicht nach NIS2 Art. 20 Abs. 2 — mind. eine dokumentierte Schulung pro Jahr.",
		Weight: 2, Ref: "NIS2 Art. 20(2)",
	},
	{
		ID: "mf.nis2.gov.review", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "governance",
		Text: "Wird die Informationssicherheitslage mind. jährlich im Management-Review überprüft?",
		HelpText: "Dokumentiertes Management-Review mit Maßnahmen, Owner und Terminen.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(a) / ISO 27001 9.3",
	},

	// ── NIS2: Risk Management (4) ───────────────────────────────────────────
	{
		ID: "mf.nis2.risk.assess", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "risk_management",
		Text: "Existiert ein dokumentierter Risikomanagement-Prozess mit aktueller Risikoliste?",
		HelpText: "5×5-Matrix oder vergleichbar, mit Owner und Maßnahme pro Risiko.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(a) / ISO 27001 6.1.2",
	},
	{
		ID: "mf.nis2.risk.treat", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "risk_management",
		Text: "Werden Risiken mit Maßnahmenplan, Termin und Verantwortlichen behandelt?",
		HelpText: "Tracker mit Status (offen / in Arbeit / abgeschlossen), regelmäßige Statusberichte.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(a) / ISO 27001 6.1.3",
	},
	{
		ID: "mf.nis2.risk.threats", Framework: FrameworkNIS2,
		Area: "risk_management",
		Text: "Werden Bedrohungen aus externen Quellen (BSI, CERT-EU, CERTBund) aufgenommen?",
		HelpText: "Mindestens quartalsweise Aktualisierung des Threat-Modells.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(a)",
	},
	{
		ID: "mf.nis2.risk.metrics", Framework: FrameworkNIS2,
		Area: "risk_management",
		Text: "Werden Risiko-Kennzahlen regelmäßig ans Management berichtet?",
		HelpText: "Risk-Score-Trend, kritische Risiken, überfällige Maßnahmen.",
		Weight: 1, Ref: "NIS2 Art. 20(1)",
	},

	// ── NIS2: Incident Response (4) ─────────────────────────────────────────
	{
		ID: "mf.nis2.ir.policy", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "incident_response",
		Text: "Existiert ein dokumentierter Incident-Response-Plan mit Rollen und Eskalationswegen?",
		HelpText: "Mit Kontakten und Test mind. jährlich — Tabletop oder Live-Übung.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(b) / ISO 27001 A.5.26",
	},
	{
		ID: "mf.nis2.ir.24h", Framework: FrameworkNIS2,
		Area: "incident_response",
		Text: "Kann eine 24h-Frühwarnung an die BSI-Meldestelle eingehalten werden?",
		HelpText: "Pflicht nach NIS2 Art. 23 + BSI-NIS2UmsG §31 — Erstmeldung spätestens 24h nach Erkenntnis.",
		Weight: 3, Ref: "NIS2 Art. 23(1)",
	},
	{
		ID: "mf.nis2.ir.72h", Framework: FrameworkNIS2,
		Area: "incident_response",
		Text: "Kann eine vollständige 72h-Incident-Meldung mit Erstbewertung erstellt werden?",
		HelpText: "Mit Schadensumfang, betroffenen Systemen und eingeleiteten Maßnahmen.",
		Weight: 2, Ref: "NIS2 Art. 23(2)",
	},
	{
		ID: "mf.nis2.ir.lessons", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "incident_response",
		Text: "Werden Lessons-Learned nach Incidents dokumentiert und umgesetzt?",
		HelpText: "Post-Incident-Review mit Verbesserungsmaßnahmen und Verantwortlichen.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(b) / ISO 27001 A.5.27",
	},

	// ── NIS2: Business Continuity (3) ───────────────────────────────────────
	{
		ID: "mf.nis2.bcm.plan", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "business_continuity",
		Text: "Existiert ein BCM/DRP mit definierten RTO/RPO pro kritischem System?",
		HelpText: "Business-Continuity- und Disaster-Recovery-Plan, dokumentiert und abgestimmt.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(c) / ISO 27001 A.5.29",
	},
	{
		ID: "mf.nis2.bcm.backup", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "business_continuity",
		Text: "Werden Backups mindestens wöchentlich auf Wiederherstellbarkeit getestet?",
		HelpText: "Test-Restore in Testumgebung, dokumentierte Ergebnisse.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(c) / ISO 27001 A.8.13",
	},
	{
		ID: "mf.nis2.bcm.test", Framework: FrameworkNIS2,
		Area: "business_continuity",
		Text: "Werden BCM-Tests mindestens jährlich mit dem Management durchgeführt?",
		HelpText: "Tabletop-Übung oder Crisis-Simulation mit Protokoll.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(c)",
	},

	// ── NIS2: Supply Chain (3) ──────────────────────────────────────────────
	{
		ID: "mf.nis2.sc.inventory", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "supply_chain",
		Text: "Existiert ein vollständiges Inventar kritischer Lieferanten mit Risikoklassifikation?",
		HelpText: "Inkl. AVV / DPA-Status pro Lieferant.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(d) / ISO 27001 A.5.19",
	},
	{
		ID: "mf.nis2.sc.assess", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "supply_chain",
		Text: "Werden Lieferanten regelmäßig auf ihre Sicherheitslage geprüft?",
		HelpText: "SOC2-Bericht, ISO 27001-Zertifikat oder eigener Sicherheitsaudit.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(d) / ISO 27001 A.5.20",
	},
	{
		ID: "mf.nis2.sc.contract", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "supply_chain",
		Text: "Enthalten Lieferantenverträge Mindest-Security-Klauseln inkl. Meldepflicht?",
		HelpText: "Vertragliche Pflicht zur Incident-Benachrichtigung und zur Einhaltung von Sicherheitsstandards.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(d) / ISO 27001 A.5.20",
	},

	// ── NIS2: Crypto (3) ────────────────────────────────────────────────────
	{
		ID: "mf.nis2.crypto.policy", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001, FrameworkDSGVOTOM},
		Area: "crypto",
		Text: "Existiert eine dokumentierte Kryptographie-Richtlinie (Algorithmen, Schlüssellängen, Key-Mgmt)?",
		HelpText: "Welche Algorithmen sind erlaubt, welche Mindeststärke, wer verwaltet Schlüssel.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(h) / ISO 27001 A.8.24 / DSGVO Art. 32(1)(a)",
	},
	{
		ID: "mf.nis2.crypto.transit", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001, FrameworkDSGVOTOM},
		Area: "crypto",
		Text: "Werden alle externen Verbindungen mit TLS 1.2+ verschlüsselt?",
		HelpText: "Kein Klartext-HTTP, kein FTP, keine Telnet für Produktivsysteme.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(h) / ISO 27001 A.8.24 / DSGVO Art. 32(1)(a)",
	},
	{
		ID: "mf.nis2.crypto.rest", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001, FrameworkDSGVOTOM},
		Area: "crypto",
		Text: "Werden personenbezogene Daten at-rest verschlüsselt gespeichert?",
		HelpText: "LUKS/dm-crypt für Disks, AES-256 für sensitive Datenbankspalten.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(h) / ISO 27001 A.8.24 / DSGVO Art. 32(1)(a)",
	},

	// ── NIS2: Access Control (4) ────────────────────────────────────────────
	{
		ID: "mf.nis2.access.mfa", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "access_control",
		Text: "Ist Multi-Faktor-Authentifizierung (MFA) für alle Admin-Accounts Pflicht?",
		HelpText: "TOTP oder Hardware-Key — keine SMS-MFA für Admin-Zugänge.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(i) / ISO 27001 A.8.5",
	},
	{
		ID: "mf.nis2.access.least", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "access_control",
		Text: "Wird das Least-Privilege-Prinzip mit dokumentierten Rollen umgesetzt?",
		HelpText: "Rollen-Matrix, mindestens halbjährliche Access-Reviews.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(i) / ISO 27001 A.8.2",
	},
	{
		ID: "mf.nis2.access.offboard", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "access_control",
		Text: "Werden Zugriffe bei Offboarding nachweislich innerhalb von 24 Stunden entzogen?",
		HelpText: "Checkliste mit Verantwortlichen und Audit-Trail.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(i) / ISO 27001 A.6.5",
	},
	{
		ID: "mf.nis2.access.review", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "access_control",
		Text: "Werden Access-Reviews mindestens halbjährlich durchgeführt und dokumentiert?",
		HelpText: "Pro Org-Rolle, mit Bestätigung durch Owner.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(i) / ISO 27001 A.8.2",
	},

	// ── NIS2: Asset Management (5) ──────────────────────────────────────────
	{
		ID: "mf.nis2.asset.inventory", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "asset_management",
		Text: "Existiert ein vollständiges, aktuelles Asset-Inventar (Hardware + Software)?",
		HelpText: "Mindestens monatliche Aktualisierung mit Owner pro Asset.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(e) / ISO 27001 A.8.1",
	},
	{
		ID: "mf.nis2.asset.patch", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "asset_management",
		Text: "Werden kritische Patches innerhalb von 30 Tagen nach Veröffentlichung eingespielt?",
		HelpText: "Dokumentierter Patch-Management-Prozess mit Reporting.",
		Weight: 3, Ref: "NIS2 Art. 21(2)(e) / ISO 27001 A.8.8",
	},
	{
		ID: "mf.nis2.asset.vuln", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "asset_management",
		Text: "Werden alle Assets regelmäßig auf Schwachstellen gescannt?",
		HelpText: "Trivy, Nuclei, OpenVAS oder vergleichbar — mindestens wöchentlich.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(e) / ISO 27001 A.8.8",
	},
	{
		ID: "mf.nis2.asset.log", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001, FrameworkDSGVOTOM},
		Area: "asset_management",
		Text: "Werden Security-Logs zentral gesammelt und mindestens 90 Tage aufbewahrt?",
		HelpText: "SIEM, ELK-Stack oder vergleichbar, mit Alerts auf Anomalien.",
		Weight: 2, Ref: "NIS2 Art. 21(2)(f) / ISO 27001 A.8.15",
	},
	{
		ID: "mf.nis2.asset.eol", Framework: FrameworkNIS2,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "asset_management",
		Text: "Werden EOL-Komponenten (End-of-Life) erkannt und zeitnah ersetzt?",
		HelpText: "endoflife.date oder vergleichbare Datenbank, Replacement-Planung dokumentiert.",
		Weight: 1, Ref: "NIS2 Art. 21(2)(e) / ISO 27001 A.8.8",
	},

	// ── ISO 27001: A.5 Organisatorische Maßnahmen (7) ───────────────────────
	{
		ID: "mf.iso.a5.isms_scope", Framework: FrameworkISO27001,
		Area: "a5_org_controls",
		Text: "Ist der ISMS-Scope (Anwendungsbereich) dokumentiert und mit der Geschäftsführung abgestimmt?",
		HelpText: "Gemäß ISO 27001 Kap. 4.3 — inkl. Abgrenzungen und Schnittstellen.",
		Weight: 2, Ref: "ISO 27001 4.3 / A.5.1",
	},
	{
		ID: "mf.iso.a5.acceptable_use", Framework: FrameworkISO27001,
		Area: "a5_org_controls",
		Text: "Existiert eine Richtlinie zur akzeptablen Nutzung von IT-Ressourcen?",
		HelpText: "Regeln für private Nutzung, Handhabung sensibler Daten, erlaubte Software.",
		Weight: 1, Ref: "ISO 27001 A.5.10",
	},
	{
		ID: "mf.iso.a5.data_classification", Framework: FrameworkISO27001,
		CrossFrameworks: []string{FrameworkDSGVOTOM},
		Area: "a5_org_controls",
		Text: "Werden Informationen nach Schutzbedarf klassifiziert (z.B. öffentlich / intern / vertraulich)?",
		HelpText: "Klassifikationsschema dokumentiert und in der Praxis angewendet.",
		Weight: 2, Ref: "ISO 27001 A.5.12 / DSGVO Art. 32",
	},
	{
		ID: "mf.iso.a5.legal_compliance", Framework: FrameworkISO27001,
		CrossFrameworks: []string{FrameworkDSGVOTOM},
		Area: "a5_org_controls",
		Text: "Werden gesetzliche und regulatorische Anforderungen systematisch identifiziert und eingehalten?",
		HelpText: "Compliance-Register mit zuständigen Personen und Fälligkeitsterminen.",
		Weight: 2, Ref: "ISO 27001 A.5.31 / DSGVO Art. 5",
	},
	{
		ID: "mf.iso.a5.supplier_security", Framework: FrameworkISO27001,
		Area: "a5_org_controls",
		Text: "Existiert eine Richtlinie für Informationssicherheit in Lieferantenbeziehungen?",
		HelpText: "Anforderungen an Dritte, Vertragsbedingungen, Auditrechte.",
		Weight: 2, Ref: "ISO 27001 A.5.19",
	},
	{
		ID: "mf.iso.a5.incident_mgmt", Framework: FrameworkISO27001,
		Area: "a5_org_controls",
		Text: "Werden Sicherheitsvorfälle nach einem definierten Prozess gemeldet, bewertet und behoben?",
		HelpText: "Klare Meldewege, Eskalationsstufen, Dokumentation und Nachverfolgung.",
		Weight: 2, Ref: "ISO 27001 A.5.26",
	},
	{
		ID: "mf.iso.a5.audit_logs", Framework: FrameworkISO27001,
		Area: "a5_org_controls",
		Text: "Werden Audit-Logs für privilegierte Zugriffe und sicherheitsrelevante Ereignisse geführt?",
		HelpText: "Schutz vor Manipulation, geregelte Aufbewahrungsfrist.",
		Weight: 2, Ref: "ISO 27001 A.8.15",
	},

	// ── ISO 27001: A.6 Personalbezogene Maßnahmen (6) ───────────────────────
	{
		ID: "mf.iso.a6.screening", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Wird vor der Einstellung eine Überprüfung des Hintergrunds (Screening) durchgeführt?",
		HelpText: "Risikobasiert: höhere Anforderungen für Positionen mit privilegiertem Zugang.",
		Weight: 1, Ref: "ISO 27001 A.6.1",
	},
	{
		ID: "mf.iso.a6.terms", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Sind Informationssicherheits-Pflichten in Arbeitsverträgen oder Anhängen geregelt?",
		HelpText: "Verschwiegenheitspflicht, Meldepflichten, Nutzungsrichtlinien.",
		Weight: 1, Ref: "ISO 27001 A.6.2",
	},
	{
		ID: "mf.iso.a6.awareness", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Erhalten alle Mitarbeitenden mindestens jährlich Sicherheitsbewusstseins-Schulungen?",
		HelpText: "Dokumentiert, mit Nachweis der Teilnahme.",
		Weight: 2, Ref: "ISO 27001 A.6.3",
	},
	{
		ID: "mf.iso.a6.disciplinary", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Existiert ein dokumentierter Prozess für disziplinarische Maßnahmen bei Sicherheitsverstößen?",
		HelpText: "Klare Regeln, faire Verfahren, Dokumentation.",
		Weight: 1, Ref: "ISO 27001 A.6.4",
	},
	{
		ID: "mf.iso.a6.offboarding", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Werden Zugangsrechte und Geräte beim Austritt vollständig und nachweislich eingezogen?",
		HelpText: "Checkliste für Offboarding mit Unterschrift und Zeitstempel.",
		Weight: 2, Ref: "ISO 27001 A.6.5",
	},
	{
		ID: "mf.iso.a6.remote_work", Framework: FrameworkISO27001,
		Area: "a6_people",
		Text: "Gibt es Richtlinien für sicheres Arbeiten im Homeoffice und auf Reisen?",
		HelpText: "VPN-Pflicht, Sperren bei Inaktivität, Umgang mit physischen Dokumenten.",
		Weight: 1, Ref: "ISO 27001 A.6.7",
	},

	// ── ISO 27001: A.8 Asset-Management (6) ─────────────────────────────────
	{
		ID: "mf.iso.a8.asset_owner", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Hat jeder Informations-Asset einen benannten Owner?",
		HelpText: "Owner ist verantwortlich für Klassifikation, Schutzmaßnahmen und Entsorgung.",
		Weight: 2, Ref: "ISO 27001 A.8.1",
	},
	{
		ID: "mf.iso.a8.media_handling", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Werden Speichermedien sicher gehandhabt, inventarisiert und bei Entsorgung sicher gelöscht?",
		HelpText: "Protokollierte Vernichtung, zertifizierter Entsorger für sensitive Medien.",
		Weight: 1, Ref: "ISO 27001 A.8.10",
	},
	{
		ID: "mf.iso.a8.secure_dev", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Werden sichere Entwicklungsprinzipien (Secure-by-Design, Code-Review, SAST) eingesetzt?",
		HelpText: "Dokumentierte Secure-SDLC-Richtlinien, mindestens für sicherheitskritische Komponenten.",
		Weight: 2, Ref: "ISO 27001 A.8.25",
	},
	{
		ID: "mf.iso.a8.config_mgmt", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Werden Konfigurationen kritischer Systeme versioniert, gehärtet und auf Drift überwacht?",
		HelpText: "Configuration-as-Code, CIS-Benchmarks oder äquivalent.",
		Weight: 2, Ref: "ISO 27001 A.8.9",
	},
	{
		ID: "mf.iso.a8.network_segregation", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Sind Netzwerke nach Schutzbedarf segmentiert (z.B. DMZ, Produktions-/Entwicklungsumgebung)?",
		HelpText: "Firewall-Regeln, VLAN-Trennung, dokumentiertes Netzwerk-Architekturdiagramm.",
		Weight: 2, Ref: "ISO 27001 A.8.22",
	},
	{
		ID: "mf.iso.a8.malware_protection", Framework: FrameworkISO27001,
		Area: "a8_asset_mgmt",
		Text: "Sind Endpoint-Protection-Lösungen (AV/EDR) auf allen relevanten Endpunkten aktiv?",
		HelpText: "Zentral verwaltet, automatische Updates, Alerting bei Erkennungen.",
		Weight: 2, Ref: "ISO 27001 A.8.7",
	},

	// ── ISO 27001: A.9 Zugriffskontrolle (6) ────────────────────────────────
	{
		ID: "mf.iso.a9.access_policy", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Existiert eine dokumentierte Zugriffskontroll-Richtlinie mit Need-to-Know-Prinzip?",
		HelpText: "Klare Kriterien, wie Zugriffsrechte vergeben, überprüft und entzogen werden.",
		Weight: 2, Ref: "ISO 27001 A.9.1 / A.8.2",
	},
	{
		ID: "mf.iso.a9.priv_access", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Werden privilegierte Zugriffe (Admin, Root, Service-Accounts) besonders gesichert und protokolliert?",
		HelpText: "PAM-Lösung oder äquivalente Kontrollen, separate Admin-Accounts.",
		Weight: 3, Ref: "ISO 27001 A.8.2",
	},
	{
		ID: "mf.iso.a9.user_provisioning", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Werden Benutzerkonten nach einem formalen Prozess angelegt, geändert und gelöscht?",
		HelpText: "Genehmigungsworkflow, Nachweisdokumentation für Audits.",
		Weight: 2, Ref: "ISO 27001 A.9.2",
	},
	{
		ID: "mf.iso.a9.password_policy", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Existiert eine Passwortrichtlinie mit Mindestlänge, Komplexität und Ablaufzeit?",
		HelpText: "Mind. 12 Zeichen für Standard-User, keine generischen Passwörter wie 'admin123'.",
		Weight: 1, Ref: "ISO 27001 A.9.4",
	},
	{
		ID: "mf.iso.a9.session_mgmt", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Werden inaktive Sessions nach einer definierten Zeit automatisch gesperrt?",
		HelpText: "Konfiguriertes Session-Timeout für alle kritischen Systeme.",
		Weight: 1, Ref: "ISO 27001 A.9.4",
	},
	{
		ID: "mf.iso.a9.access_review_periodic", Framework: FrameworkISO27001,
		Area: "a9_access_control",
		Text: "Werden alle Zugriffsrechte mindestens jährlich durch die jeweiligen Verantwortlichen überprüft?",
		HelpText: "Formaler Review-Prozess mit Bestätigung oder Entzug pro Account.",
		Weight: 2, Ref: "ISO 27001 A.9.5",
	},

	// ── DSGVO-TOM: Datenschutz-Grundsätze (8) ───────────────────────────────
	{
		ID: "mf.dsgvo.grundsatz.zweck", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Werden personenbezogene Daten nur zu festgelegten, eindeutigen und legitimen Zwecken verarbeitet?",
		HelpText: "Zweckbindungsgrundsatz Art. 5(1)(b) — kein Umwidmen ohne neue Rechtsgrundlage.",
		Weight: 2, Ref: "DSGVO Art. 5(1)(b)",
	},
	{
		ID: "mf.dsgvo.grundsatz.datensparsamkeit", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Wird Datenminimierung konsequent umgesetzt — es werden nur die tatsächlich notwendigen Daten erhoben?",
		HelpText: "Datensparsamkeitsprinzip: keine übermäßige Erfassung, keine Vorratsdatenhaltung.",
		Weight: 2, Ref: "DSGVO Art. 5(1)(c)",
	},
	{
		ID: "mf.dsgvo.grundsatz.richtigkeit", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Werden Prozesse zur Sicherstellung der Datenkorrektheit und Aktualisierung betrieben?",
		HelpText: "Regelmäßige Datenbereinigung, Korrekturmechanismen für Betroffene.",
		Weight: 1, Ref: "DSGVO Art. 5(1)(d)",
	},
	{
		ID: "mf.dsgvo.grundsatz.speicherbegrenzung", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Existieren Lösch- und Aufbewahrungsfristen für alle Kategorien personenbezogener Daten?",
		HelpText: "Löschkonzept mit definierten Fristen, automatisierter oder manuell geplanter Löschung.",
		Weight: 2, Ref: "DSGVO Art. 5(1)(e)",
	},
	{
		ID: "mf.dsgvo.grundsatz.rechtsgrundlage", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Hat jede Datenverarbeitung eine dokumentierte Rechtsgrundlage (Einwilligung, Vertrag, berechtigtes Interesse etc.)?",
		HelpText: "Verzeichnis von Verarbeitungstätigkeiten (Art. 30) mit Rechtsgrundlage pro Verarbeitung.",
		Weight: 3, Ref: "DSGVO Art. 6 / Art. 30",
	},
	{
		ID: "mf.dsgvo.grundsatz.betroffenenrechte", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Können Auskunfts-, Berichtigungs-, Lösch- und Widerspruchsanfragen fristgerecht bearbeitet werden?",
		HelpText: "Prozess für Betroffenenanfragen mit 30-Tage-Frist (Art. 12), dokumentierte Bearbeitung.",
		Weight: 2, Ref: "DSGVO Art. 12–22",
	},
	{
		ID: "mf.dsgvo.grundsatz.dsb", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Ist ein Datenschutzbeauftragter (DSB) benannt und der Aufsichtsbehörde gemeldet (sofern verpflichtend)?",
		HelpText: "Pflicht nach Art. 37 für öffentliche Stellen, Kerntätigkeit mit umfangreicher Verarbeitung oder besonderer Kategorien.",
		Weight: 2, Ref: "DSGVO Art. 37–39",
	},
	{
		ID: "mf.dsgvo.grundsatz.vvt", Framework: FrameworkDSGVOTOM,
		Area: "datenschutz_grundsaetze",
		Text: "Ist ein vollständiges Verzeichnis von Verarbeitungstätigkeiten (VVT) vorhanden und aktuell?",
		HelpText: "Art. 30 — für jede Verarbeitung: Zweck, Rechtsgrundlage, Kategorien, Löschfrist, Empfänger.",
		Weight: 2, Ref: "DSGVO Art. 30",
	},

	// ── DSGVO-TOM: Technische Schutzmaßnahmen (9) ───────────────────────────
	{
		ID: "mf.dsgvo.tom.verschluesselung", Framework: FrameworkDSGVOTOM,
		CrossFrameworks: []string{FrameworkNIS2, FrameworkISO27001},
		Area: "technische_schutzmassnahmen",
		Text: "Werden personenbezogene Daten sowohl bei der Übertragung als auch bei der Speicherung verschlüsselt?",
		HelpText: "TLS 1.2+ für Übertragung, AES-256 für Speicherung sensibler Daten.",
		Weight: 3, Ref: "DSGVO Art. 32(1)(a) / NIS2 Art. 21(2)(h)",
	},
	{
		ID: "mf.dsgvo.tom.pseudonymisierung", Framework: FrameworkDSGVOTOM,
		Area: "technische_schutzmassnahmen",
		Text: "Wird Pseudonymisierung für personenbezogene Daten in Analyse- und Test-Umgebungen eingesetzt?",
		HelpText: "Test-Daten dürfen keine echten personenbezogenen Daten enthalten ohne Pseudonymisierung.",
		Weight: 2, Ref: "DSGVO Art. 32(1)(a)",
	},
	{
		ID: "mf.dsgvo.tom.zugangsprotokoll", Framework: FrameworkDSGVOTOM,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "technische_schutzmassnahmen",
		Text: "Werden Zugriffe auf Systeme mit personenbezogenen Daten protokolliert und ausgewertet?",
		HelpText: "Zugriffsprotokoll mit Timestamp, User-ID, System — Aufbewahrung mind. 90 Tage.",
		Weight: 2, Ref: "DSGVO Art. 32 / ISO 27001 A.8.15",
	},
	{
		ID: "mf.dsgvo.tom.verfuegbarkeit", Framework: FrameworkDSGVOTOM,
		CrossFrameworks: []string{FrameworkNIS2},
		Area: "technische_schutzmassnahmen",
		Text: "Sind Maßnahmen zur Sicherstellung der Verfügbarkeit und Widerherstellbarkeit von Systemen und Daten implementiert?",
		HelpText: "Backups, Redundanzen, Failover — mit dokumentierten Wiederherstellungszeiten (RTO/RPO).",
		Weight: 2, Ref: "DSGVO Art. 32(1)(c) / NIS2 Art. 21(2)(c)",
	},
	{
		ID: "mf.dsgvo.tom.datenpannen", Framework: FrameworkDSGVOTOM,
		Area: "technische_schutzmassnahmen",
		Text: "Existiert ein Prozess zur Erkennung, Dokumentation und Meldung von Datenpannen innerhalb von 72 Stunden?",
		HelpText: "Pflicht nach Art. 33 — Meldung an Aufsichtsbehörde, ggf. Benachrichtigung Betroffener (Art. 34).",
		Weight: 3, Ref: "DSGVO Art. 33–34",
	},
	{
		ID: "mf.dsgvo.tom.privacy_by_design", Framework: FrameworkDSGVOTOM,
		Area: "technische_schutzmassnahmen",
		Text: "Werden Datenschutzanforderungen bereits in der Entwicklungsphase neuer Systeme berücksichtigt (Privacy by Design)?",
		HelpText: "Art. 25 — Datenschutzfreundliche Voreinstellungen, Einbindung des DSB bei neuen Produktfeatures.",
		Weight: 2, Ref: "DSGVO Art. 25",
	},
	{
		ID: "mf.dsgvo.tom.dsfa", Framework: FrameworkDSGVOTOM,
		Area: "technische_schutzmassnahmen",
		Text: "Wurden Datenschutz-Folgenabschätzungen (DSFA) für risikoreich Verarbeitungen durchgeführt?",
		HelpText: "Art. 35 — erforderlich für hohe Risiken wie umfangreiche Verarbeitung sensibler Daten, Profiling, Videoüberwachung.",
		Weight: 2, Ref: "DSGVO Art. 35",
	},
	{
		ID: "mf.dsgvo.tom.zugriffsbeschraenkung", Framework: FrameworkDSGVOTOM,
		CrossFrameworks: []string{FrameworkISO27001},
		Area: "technische_schutzmassnahmen",
		Text: "Ist der Zugang zu personenbezogenen Daten auf das zur Aufgabenerfüllung notwendige Minimum beschränkt?",
		HelpText: "Role-Based Access Control mit regelmäßigem Review.",
		Weight: 2, Ref: "DSGVO Art. 32 / ISO 27001 A.8.2",
	},
	{
		ID: "mf.dsgvo.tom.integritaet", Framework: FrameworkDSGVOTOM,
		Area: "technische_schutzmassnahmen",
		Text: "Sind Maßnahmen zur Sicherstellung der Integrität von personenbezogenen Daten implementiert?",
		HelpText: "Checksummen, signierte Datenimporte, Schutz vor unbefugter Manipulation.",
		Weight: 1, Ref: "DSGVO Art. 32(1)(b)",
	},

	// ── DSGVO-TOM: Auftragsverarbeitung (8) ─────────────────────────────────
	{
		ID: "mf.dsgvo.avv.inventar", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Existiert ein vollständiges Inventar aller Auftragsverarbeiter (Art. 28) mit aktuellem AVV?",
		HelpText: "Jeder Dienstleister, der in Ihrem Auftrag personenbezogene Daten verarbeitet, benötigt einen AVV.",
		Weight: 3, Ref: "DSGVO Art. 28(1)",
	},
	{
		ID: "mf.dsgvo.avv.inhalt", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Enthalten alle AVVs die gesetzlich vorgeschriebenen Mindestinhalte (Art. 28 Abs. 3)?",
		HelpText: "Gegenstand, Dauer, Zweck, Weisungsbindung, Sub-AV-Regelung, Audit-Rechte.",
		Weight: 2, Ref: "DSGVO Art. 28(3)",
	},
	{
		ID: "mf.dsgvo.avv.sub_auftragsverarbeiter", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Werden Sub-Auftragsverarbeiter dokumentiert und genehmigt, bevor Daten an sie weitergegeben werden?",
		HelpText: "Genehmigungspflicht (spezifisch oder generell) nach Art. 28(2).",
		Weight: 2, Ref: "DSGVO Art. 28(2)",
	},
	{
		ID: "mf.dsgvo.avv.drittland", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Werden Datentransfers in Drittländer (außerhalb EU/EWR) rechtlich abgesichert?",
		HelpText: "Standardvertragsklauseln (SCCs), Angemessenheitsbeschluss oder BCRs.",
		Weight: 3, Ref: "DSGVO Art. 44–49",
	},
	{
		ID: "mf.dsgvo.avv.kontrolle", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Werden Auftragsverarbeiter regelmäßig auf Einhaltung der Datenschutzanforderungen kontrolliert?",
		HelpText: "Audit-Rechte, Sicherheitsnachweise (ISO 27001, SOC2), regelmäßige Überprüfung.",
		Weight: 2, Ref: "DSGVO Art. 28(3)(h)",
	},
	{
		ID: "mf.dsgvo.avv.weisungen", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Werden Weisungen an Auftragsverarbeiter dokumentiert und auf Einhaltung geprüft?",
		HelpText: "Jede Verarbeitung außerhalb der Weisung des Verantwortlichen ist unzulässig.",
		Weight: 1, Ref: "DSGVO Art. 28(3)(a)",
	},
	{
		ID: "mf.dsgvo.avv.abschluss_vor_start", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Wird vor Beginn jeder neuen Auftragsverarbeitung ein AVV abgeschlossen?",
		HelpText: "Kein Produktivbetrieb ohne unterschriebenen AVV — auch bei Cloud-Diensten.",
		Weight: 2, Ref: "DSGVO Art. 28(1)",
	},
	{
		ID: "mf.dsgvo.avv.loeschung_rueckgabe", Framework: FrameworkDSGVOTOM,
		Area: "auftragsverarbeitung",
		Text: "Ist sichergestellt, dass Auftragsverarbeiter nach Vertragsende alle Daten löschen oder zurückgeben?",
		HelpText: "Vertraglich geregelt, Nachweis über tatsächliche Löschung anfordern.",
		Weight: 1, Ref: "DSGVO Art. 28(3)(g)",
	},
}

// MultiFrameworkQuestionCount gibt die Gesamtanzahl der Multi-Framework-Fragen zurück.
func MultiFrameworkQuestionCount() int { return len(MultiFrameworkQuestions) }

// validMultiFrameworkQuestionID prüft gegen die MultiFrameworkQuestions-Liste.
func validMultiFrameworkQuestionID(id string) bool {
	for _, q := range MultiFrameworkQuestions {
		if q.ID == id {
			return true
		}
	}
	return false
}
