package nis2wizard

// Sprint 19 / S19-2: 30-Fragen-NIS2-Self-Assessment.
//
// Quelle: NIS2-Direktive Art. 21 + BSI NIS2-Umsetzungsgesetz §30. Acht
// Themenbereiche, je 3-5 Fragen. Skala 0-4 (0 = "nicht implementiert" bis
// 4 = "vollständig + nachgewiesen + getestet"). Optionaler Freitext.

// Area gruppiert verwandte Fragen für den Score-Output.
type Area string

const (
	AreaGovernance       Area = "governance"
	AreaRiskManagement   Area = "risk_management"
	AreaIncidentResponse Area = "incident_response"
	AreaBusinessCont     Area = "business_continuity"
	AreaSupplyChain      Area = "supply_chain"
	AreaCrypto           Area = "crypto"
	AreaAccessControl    Area = "access_control"
	AreaAssetMgmt        Area = "asset_management"
)

// AllAreas in Reihenfolge der Anzeige.
var AllAreas = []Area{
	AreaGovernance, AreaRiskManagement, AreaIncidentResponse, AreaBusinessCont,
	AreaSupplyChain, AreaCrypto, AreaAccessControl, AreaAssetMgmt,
}

// Question beschreibt eine einzelne Wizard-Frage.
type Question struct {
	ID       string `json:"id"`        // stable z.B. "gov.policy"
	Area     Area   `json:"area"`
	Title    string `json:"title"`     // kurze Frage
	Help     string `json:"help"`      // 1-2 Sätze Hilfetext
	NIS2Ref  string `json:"nis2_ref"`  // Verweis auf NIS2-Art. § Buchst.
	Weight   int    `json:"weight"`    // 1-3, Score-Gewichtung
}

// Questions ist die 30-Fragen-Liste. Beim Edit: stable IDs nicht ändern,
// sonst brechen historische Assessments im Pro-Trend-View.
var Questions = []Question{
	// --- Governance (4) ---
	{ID: "gov.policy", Area: AreaGovernance, Title: "Existiert eine schriftliche Informationssicherheits-Leitlinie?",
		Help: "Vom Top-Management freigegeben, mindestens einmal im Jahr überprüft.", NIS2Ref: "Art. 21 Abs. 2 a)", Weight: 3},
	{ID: "gov.responsibility", Area: AreaGovernance, Title: "Ist ein CISO oder Sicherheitsverantwortlicher benannt?",
		Help: "Mit klaren Verantwortlichkeiten + Berichtsweg zum Vorstand/Geschäftsführung.", NIS2Ref: "Art. 20 Abs. 1", Weight: 2},
	{ID: "gov.training", Area: AreaGovernance, Title: "Erhält die Geschäftsleitung NIS2-spezifische Schulungen?",
		Help: "Pflicht nach NIS2 Art. 20 Abs. 2 — mind. jährlich.", NIS2Ref: "Art. 20 Abs. 2", Weight: 2},
	{ID: "gov.review", Area: AreaGovernance, Title: "Wird die Informationssicherheits-Lage mind. jährlich reviewed?",
		Help: "Mit dokumentierten Maßnahmen + Owner + Frist pro Befund.", NIS2Ref: "Art. 21 Abs. 2 a)", Weight: 1},

	// --- Risk Management (4) ---
	{ID: "risk.assess", Area: AreaRiskManagement, Title: "Existiert ein Risikomanagement-Prozess mit dokumentierten Risiken?",
		Help: "5x5-Matrix oder vergleichbar, mit Owner + Maßnahme pro Risiko.", NIS2Ref: "Art. 21 Abs. 2 a)", Weight: 3},
	{ID: "risk.treat", Area: AreaRiskManagement, Title: "Werden Risiken mit Maßnahmenplan + Termin behandelt?",
		Help: "Tracker mit Status (offen/in Arbeit/abgeschlossen).", NIS2Ref: "Art. 21 Abs. 2 a)", Weight: 2},
	{ID: "risk.threats", Area: AreaRiskManagement, Title: "Werden Bedrohungen aus externen Quellen aufgenommen (BSI, CERT-EU)?",
		Help: "Mind. quartärliche Aktualisierung des Threat-Modells.", NIS2Ref: "Art. 21 Abs. 2 a)", Weight: 1},
	{ID: "risk.metrics", Area: AreaRiskManagement, Title: "Werden Risiko-Kennzahlen ans Management berichtet?",
		Help: "Risk-Score-Trend, kritische Risiken, überfällige Maßnahmen.", NIS2Ref: "Art. 20 Abs. 1", Weight: 1},

	// --- Incident Response (4) ---
	{ID: "ir.policy", Area: AreaIncidentResponse, Title: "Existiert ein dokumentierter Incident-Response-Plan?",
		Help: "Mit Rollen, Eskalationswegen, Kontakten — getestet mind. jährlich.", NIS2Ref: "Art. 21 Abs. 2 b)", Weight: 3},
	{ID: "ir.24h", Area: AreaIncidentResponse, Title: "Kann eine 24h-Frühwarnung an die BSI-Meldestelle eingehalten werden?",
		Help: "Pflicht nach NIS2 Art. 23 + BSI-NIS2UmsG §31.", NIS2Ref: "Art. 23 Abs. 1", Weight: 3},
	{ID: "ir.72h", Area: AreaIncidentResponse, Title: "Kann eine 72h-Incident-Meldung mit Erstbewertung eingehalten werden?",
		Help: "Mit Schadensumfang + Maßnahmen.", NIS2Ref: "Art. 23 Abs. 2", Weight: 2},
	{ID: "ir.lessons", Area: AreaIncidentResponse, Title: "Werden Lessons-Learned nach Incidents dokumentiert?",
		Help: "Mit Verbesserungsmaßnahmen + Verantwortlichen.", NIS2Ref: "Art. 21 Abs. 2 b)", Weight: 1},

	// --- Business Continuity (3) ---
	{ID: "bcm.plan", Area: AreaBusinessCont, Title: "Existiert ein BCM/DRP (Business-Continuity / Disaster-Recovery-Plan)?",
		Help: "Mit definierten RTO/RPO pro kritischem System.", NIS2Ref: "Art. 21 Abs. 2 c)", Weight: 3},
	{ID: "bcm.backup", Area: AreaBusinessCont, Title: "Werden Backups mind. wöchentlich getestet wiederhergestellt?",
		Help: "Test-Restore in Test-Umgebung, dokumentiert.", NIS2Ref: "Art. 21 Abs. 2 c)", Weight: 2},
	{ID: "bcm.test", Area: AreaBusinessCont, Title: "Werden BCM-Tests mind. jährlich mit dem Management durchgeführt?",
		Help: "Tabletop-Übung oder Crisis-Simulation.", NIS2Ref: "Art. 21 Abs. 2 c)", Weight: 1},

	// --- Supply Chain (3) ---
	{ID: "sc.inventory", Area: AreaSupplyChain, Title: "Existiert ein vollständiges Inventar kritischer Lieferanten?",
		Help: "Mit Risiko-Klassifikation + AVV.", NIS2Ref: "Art. 21 Abs. 2 d)", Weight: 3},
	{ID: "sc.assess", Area: AreaSupplyChain, Title: "Werden Lieferanten regelmäßig auf Sicherheitslage geprüft?",
		Help: "SOC2-Bericht, ISO27001-Zertifikat oder eigener Audit.", NIS2Ref: "Art. 21 Abs. 2 d)", Weight: 2},
	{ID: "sc.contract", Area: AreaSupplyChain, Title: "Enthalten Lieferantenverträge Mindest-Security-Klauseln?",
		Help: "Inkl. Notification-Pflicht bei Incidents.", NIS2Ref: "Art. 21 Abs. 2 d)", Weight: 1},

	// --- Crypto (3) ---
	{ID: "crypto.policy", Area: AreaCrypto, Title: "Existiert eine dokumentierte Kryptographie-Richtlinie?",
		Help: "Welche Algorithmen, welche Mindeststärke, Key-Mgmt-Prozess.", NIS2Ref: "Art. 21 Abs. 2 h)", Weight: 2},
	{ID: "crypto.transit", Area: AreaCrypto, Title: "Werden alle externen Verbindungen mit TLS 1.2+ verschlüsselt?",
		Help: "Kein Klartext-HTTP, kein FTP, keine Telnet.", NIS2Ref: "Art. 21 Abs. 2 h)", Weight: 3},
	{ID: "crypto.rest", Area: AreaCrypto, Title: "Werden personenbezogene Daten at-rest verschlüsselt?",
		Help: "LUKS/dm-crypt für Disks, AES-256 für sensitive DB-Spalten.", NIS2Ref: "Art. 21 Abs. 2 h)", Weight: 2},

	// --- Access Control (4) ---
	{ID: "access.mfa", Area: AreaAccessControl, Title: "Ist MFA für alle Admin-Accounts Pflicht?",
		Help: "TOTP oder Hardware-Key, keine SMS-MFA für Admins.", NIS2Ref: "Art. 21 Abs. 2 i)", Weight: 3},
	{ID: "access.least", Area: AreaAccessControl, Title: "Wird das Least-Privilege-Prinzip mit dokumentierten Rollen umgesetzt?",
		Help: "Rollen-Matrix, mind. halbjährliche Access-Reviews.", NIS2Ref: "Art. 21 Abs. 2 i)", Weight: 2},
	{ID: "access.offboard", Area: AreaAccessControl, Title: "Werden Zugriffe bei Offboarding innerhalb 24h entzogen?",
		Help: "Checkliste, Verantwortliche, Audit-Trail.", NIS2Ref: "Art. 21 Abs. 2 i)", Weight: 2},
	{ID: "access.review", Area: AreaAccessControl, Title: "Werden Access-Reviews mind. halbjährlich durchgeführt?",
		Help: "Pro Org-Rolle, mit Bestätigung durch Owner.", NIS2Ref: "Art. 21 Abs. 2 i)", Weight: 1},

	// --- Asset Management (5) ---
	{ID: "asset.inventory", Area: AreaAssetMgmt, Title: "Existiert ein vollständiges Asset-Inventar (Hardware + Software)?",
		Help: "Mind. monatliche Aktualisierung, mit Owner pro Asset.", NIS2Ref: "Art. 21 Abs. 2 e)", Weight: 3},
	{ID: "asset.patch", Area: AreaAssetMgmt, Title: "Werden kritische Patches innerhalb 30 Tagen eingespielt?",
		Help: "Dokumentierter Patch-Mgmt-Prozess + Reporting.", NIS2Ref: "Art. 21 Abs. 2 e)", Weight: 3},
	{ID: "asset.vuln", Area: AreaAssetMgmt, Title: "Werden alle Assets regelmäßig auf Vulnerabilities gescannt?",
		Help: "Trivy, Nuclei, OpenVAS oder vergleichbar — mind. wöchentlich.", NIS2Ref: "Art. 21 Abs. 2 e)", Weight: 2},
	{ID: "asset.log", Area: AreaAssetMgmt, Title: "Werden Security-Logs zentral gesammelt + min. 90 Tage aufbewahrt?",
		Help: "SIEM, ELK-Stack oder vergleichbar, mit Alerts auf Anomalien.", NIS2Ref: "Art. 21 Abs. 2 f)", Weight: 2},
	{ID: "asset.eol", Area: AreaAssetMgmt, Title: "Werden EOL-Komponenten erkannt und ersetzt?",
		Help: "endoflife.date oder vergleichbare Datenbank.", NIS2Ref: "Art. 21 Abs. 2 e)", Weight: 1},
}

// AreaTitle liefert den User-facing Bereichs-Titel.
func AreaTitle(a Area) string {
	switch a {
	case AreaGovernance:
		return "Governance & Verantwortlichkeit"
	case AreaRiskManagement:
		return "Risikomanagement"
	case AreaIncidentResponse:
		return "Incident-Response"
	case AreaBusinessCont:
		return "Business Continuity"
	case AreaSupplyChain:
		return "Lieferketten-Sicherheit"
	case AreaCrypto:
		return "Kryptographie"
	case AreaAccessControl:
		return "Zugriffskontrolle"
	case AreaAssetMgmt:
		return "Asset- & Vulnerability-Mgmt"
	default:
		return string(a)
	}
}
