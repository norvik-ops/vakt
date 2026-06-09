// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// --- Internal helpers ---

// computeReadinessReport calculates readiness metrics given controls and evidence counts.
func computeReadinessReport(fw *Framework, controls []Control, evidenceCounts map[string]int) *ReadinessReport {
	report := &ReadinessReport{
		FrameworkID:   fw.ID,
		FrameworkName: fw.Name,
		TotalControls: len(controls),
	}

	// Per-domain tracking.
	domainTotal := make(map[string]int)
	domainCovered := make(map[string]int)

	for _, c := range controls {
		count := evidenceCounts[c.ID]
		domainTotal[c.Domain]++

		switch {
		case count >= 2:
			report.Covered++
			domainCovered[c.Domain]++
		case count == 1:
			report.Partial++
			domainCovered[c.Domain]++ // partial counts as half for domain score
		default:
			report.Missing++
		}
	}

	// Overall readiness score.
	if report.TotalControls > 0 {
		report.ReadinessScore = readinessScore(report.Covered, report.Partial, report.TotalControls)
	}

	// Per-domain scores.
	for domain, total := range domainTotal {
		if total == 0 {
			continue
		}
		covered := domainCovered[domain]
		score := readinessScore(covered, 0, total)
		report.ByDomain = append(report.ByDomain, DomainScore{
			Domain:  domain,
			Score:   score,
			Total:   total,
			Covered: covered,
		})
	}

	return report
}

// readinessScore calculates a 0–100 readiness score.
// Covered controls count fully; partial controls count as half-weight.
func readinessScore(covered, partial, total int) float64 {
	if total == 0 {
		return 0
	}
	weighted := float64(covered) + float64(partial)*0.5
	return (weighted / float64(total)) * 100
}

// resolveStatus determines the effective status of a control.
// Priority: not_applicable > manual_status > computed from evidence.
func resolveStatus(c Control) string {
	if c.NotApplicable {
		return "not_applicable"
	}
	if c.ManualStatus != "" {
		return c.ManualStatus
	}
	return controlStatus(c.EvidenceCount)
}

// controlStatus returns a computed coverage label for a control.
func controlStatus(evidenceCount int) string {
	switch {
	case evidenceCount >= 2:
		return "covered"
	case evidenceCount == 1:
		return "partial"
	default:
		return "missing"
	}
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}
	rawToken = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(sum[:])
	return rawToken, tokenHash, nil
}

// --- Built-in framework templates ---

// builtinVersion returns the canonical version string for a well-known framework name.
func builtinVersion(name string) string {
	versions := map[string]string{
		"NIS2":      "2022",
		"ISO27001":  "2022",
		"BSI":       "2023",
		"CRA":       "2024",
		"DORA":      "2022",
		"EUAIACT":   "2024",
		"ISO42001":  "2023",
		"TISAX":     "6.0",
		"DSGVO-TOM": "2018",
		"CIS":       "8.1",
	}
	return versions[name]
}

// builtinControls seeds a small set of representative controls for well-known frameworks.
// In production expand or load from embedded JSON/CSV files.
func builtinControls(frameworkID, orgID, name string) []Control {
	switch name {
	case "NIS2":
		return nis2Controls(frameworkID, orgID)
	case "ISO27001":
		return iso27001Controls(frameworkID, orgID)
	case "BSI":
		return bsiControls(frameworkID, orgID)
	case "CRA":
		return craControls(frameworkID, orgID)
	case "DORA":
		return doraControls(frameworkID, orgID)
	case "EUAIACT":
		return euAiActControls(frameworkID, orgID)
	case "ISO42001":
		return iso42001Controls(frameworkID, orgID)
	case "TISAX":
		return tisaxControls(frameworkID, orgID)
	case "DSGVO-TOM":
		return dsgvoTOMControls(frameworkID, orgID)
	case "CIS":
		return cisControls(frameworkID, orgID)
	}
	return nil
}

func nis2Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 21(2)(a) — Risikomanagement
		c("NIS2-A.1", "Informationssicherheitsrichtlinie",
			"Erstelle und genehmige eine schriftliche Informationssicherheitsrichtlinie. Sie muss Schutzziele, Geltungsbereich, Verantwortlichkeiten und Überprüfungsintervall enthalten. Nachweis: unterschriebenes Richtliniendokument mit Versionsnummer und Genehmigungsdatum.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.2", "Risikomanagement-Framework",
			"Implementiere einen formalen Prozess zur Identifikation, Bewertung und Behandlung von Informationssicherheitsrisiken. Nachweis: Risikomanagement-Prozessbeschreibung, Risikoregister.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.3", "Risikoanalyse und -bewertung",
			"Führe mindestens jährlich eine strukturierte Risikoanalyse durch. Bewerte Eintrittswahrscheinlichkeit und Auswirkung für alle relevanten Bedrohungen. Nachweis: ausgefülltes Risikoregister mit Bewertungsmatrix.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.4", "Risikobehandlungsplan",
			"Definiere für alle identifizierten Risiken konkrete Maßnahmen (Vermeiden, Reduzieren, Übertragen, Akzeptieren) mit Verantwortlichen und Fristen. Nachweis: Risikobehandlungsplan mit Umsetzungsstatus.",
			"Risikomanagement", "manual", 3),
		c("NIS2-A.5", "Sicherheitsziele und Governance",
			"Lege messbare Sicherheitsziele auf Organisations- und Abteilungsebene fest. Stelle sicher, dass die Geschäftsführung die IS-Governance trägt. Nachweis: dokumentierte Sicherheitsziele, Protokolle von Management-Reviews.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.6", "Rollen und Verantwortlichkeiten IS",
			"Benenne einen Informationssicherheitsbeauftragten (ISB) und dokumentiere alle sicherheitsrelevanten Rollen und deren Verantwortlichkeiten. Nachweis: Organigramm, Stellenbeschreibungen, Beauftragungsschreiben.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.7", "Richtlinienüberprüfung und Genehmigung",
			"Überprüfe alle Sicherheitsrichtlinien mindestens jährlich oder nach wesentlichen Änderungen und hole erneute Genehmigung ein. Nachweis: Änderungshistorie der Richtlinien mit Genehmigungsnachweisen.",
			"Risikomanagement", "manual", 1),
		c("NIS2-A.8", "Asset-Inventar und Klassifizierung",
			"Führe ein aktuelles Inventar aller informationsverarbeitenden Assets (Hardware, Software, Daten). Klassifiziere Assets nach Schutzbedarf. Nachweis: Asset-Register mit Klassifizierungsschema.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.9", "Bedrohungsanalyse (Threat Intelligence)",
			"Abonniere relevante Bedrohungsinformationen (CERT-Bund, BSI-Warnmeldungen, CVE-Feeds) und integriere sie in den Risikoprozess. Nachweis: Abonnementbestätigung, Prozessdokumentation zur Verarbeitung.",
			"Risikomanagement", "manual", 2),
		c("NIS2-A.10", "Compliance-Management",
			"Identifiziere alle anwendbaren gesetzlichen, regulatorischen und vertraglichen Anforderungen (NIS2, DSGVO, branchenspezifisch) und verfolge deren Einhaltung. Nachweis: Compliance-Register, Auditberichte.",
			"Risikomanagement", "manual", 2),

		// Art. 21(2)(b) — Incident Handling
		c("NIS2-B.1", "Incident-Response-Richtlinie",
			"Erstelle eine schriftliche Incident-Response-Richtlinie mit Klassifizierungsschema, Eskalationspfaden und Reaktionszeiten. Nachweis: genehmigtes Richtliniendokument.",
			"Incident Management", "manual", 3),
		c("NIS2-B.2", "Erkennung und Überwachung von Vorfällen",
			"Implementiere technische Erkennungsmechanismen (SIEM, IDS, Log-Monitoring). Stelle sicher, dass Alarme rund um die Uhr überwacht werden. Nachweis: SIEM-Konfiguration, Monitoring-Dashboard.",
			"Incident Management", "automated", 3),
		c("NIS2-B.3", "Incident-Response-Team (CSIRT)",
			"Bilde ein benanntes Incident-Response-Team mit klaren Rollen. Stelle Erreichbarkeit und Eskalationspfade sicher. Nachweis: Teambesetzungsplan, Kontaktliste, Beauftragungsschreiben.",
			"Incident Management", "manual", 2),
		c("NIS2-B.4", "Klassifizierung und Priorisierung von Vorfällen",
			"Definiere ein Klassifizierungsschema (Schweregrade 1–4 o.ä.) mit konkreten Kriterien und daraus abgeleiteten Reaktionszeiten. Nachweis: Klassifizierungsmatrix im Incident-Response-Plan.",
			"Incident Management", "manual", 2),
		c("NIS2-B.5", "Meldung an Behörde innerhalb 24 Stunden",
			"Stelle sicher, dass erhebliche Sicherheitsvorfälle gem. Art. 23 NIS2 innerhalb von 24 Stunden an das BSI/zuständige CSIRT gemeldet werden. Nachweis: Meldeprozess-Dokumentation, ggf. Muster-Meldung.",
			"Incident Management", "manual", 3),
		c("NIS2-B.6", "Detaillierter Vorfallsbericht innerhalb 72 Stunden",
			"Erstelle innerhalb von 72 Stunden nach Ersterkennung einen detaillierten Vorfallsbericht an die Aufsichtsbehörde. Nachweis: Berichtsvorlage, Eskalationsplan mit Fristen.",
			"Incident Management", "manual", 3),
		c("NIS2-B.7", "Post-Incident-Review",
			"Führe nach jedem erheblichen Vorfall eine strukturierte Nachbesprechung (Post-Mortem) durch und dokumentiere Erkenntnisse und Verbesserungsmaßnahmen. Nachweis: Post-Incident-Review-Berichte.",
			"Incident Management", "manual", 2),
		c("NIS2-B.8", "Kommunikations- und Eskalationsplan",
			"Dokumentiere interne und externe Kommunikationswege für den Krisenfall inkl. Pressestelle, Juristen, Behörden. Nachweis: Kommunikationsplan mit Kontaktlisten.",
			"Incident Management", "manual", 2),
		c("NIS2-B.9", "Forensische Beweissicherung",
			"Definiere Verfahren zur gerichtsfesten Sicherung digitaler Beweise bei Vorfällen. Stelle notwendige Tools und Schulung bereit. Nachweis: Forensik-Checkliste, Tool-Dokumentation.",
			"Incident Management", "manual", 1),

		// Art. 21(2)(c) — Business Continuity
		c("NIS2-C.1", "Business-Continuity-Richtlinie",
			"Erstelle eine BCM-Richtlinie, die Geltungsbereich, Verantwortlichkeiten und Ziele des Business-Continuity-Managements festlegt. Nachweis: genehmigtes BCM-Richtliniendokument.",
			"Business Continuity", "manual", 2),
		c("NIS2-C.2", "Business Impact Analysis (BIA)",
			"Analysiere alle kritischen Geschäftsprozesse hinsichtlich Auswirkung und maximaler Ausfallzeit. Nachweis: BIA-Dokument mit MTPD und MBCO-Angaben.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.3", "RTO/RPO-Ziele definiert",
			"Lege für alle kritischen Systeme konkrete Recovery Time Objectives (RTO) und Recovery Point Objectives (RPO) fest. Nachweis: RTO/RPO-Tabelle, abgestimmt mit BIA.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.4", "Backup-Richtlinie und -Verfahren",
			"Definiere Backup-Häufigkeit, Aufbewahrungsdauer, Speicherort (3-2-1-Regel) und Verschlüsselung. Nachweis: Backup-Richtlinie, Backup-Job-Konfiguration.",
			"Business Continuity", "automated", 3),
		c("NIS2-C.5", "Backup-Tests und -Überprüfung",
			"Teste Backups mindestens vierteljährlich durch tatsächliche Wiederherstellung. Dokumentiere Ergebnisse. Nachweis: Backup-Testberichte mit Datum und Ergebnis.",
			"Business Continuity", "automated", 3),
		c("NIS2-C.6", "Notfallwiederherstellungsplan (DR)",
			"Erstelle einen detaillierten Disaster-Recovery-Plan mit konkreten Wiederherstellungsschritten je Kritisch-System. Nachweis: DR-Plan-Dokument.",
			"Business Continuity", "manual", 3),
		c("NIS2-C.7", "DR-Tests und -Übungen",
			"Führe mindestens jährlich einen DR-Test (Tabletop-Übung oder Live-Test) durch. Nachweis: Übungsprotokoll mit Ergebnissen und Verbesserungsmaßnahmen.",
			"Business Continuity", "manual", 2),
		c("NIS2-C.8", "Krisenkommunkationsplan",
			"Dokumentiere Kommunikationswege für den Krisenfall: interne Benachrichtigung, externe Kommunikation (Kunden, Medien, Behörden). Nachweis: Kommunikationsplan.",
			"Business Continuity", "manual", 1),
		c("NIS2-C.9", "Redundanz und Hochverfügbarkeit",
			"Implementiere technische Redundanz für kritische Systeme (Failover, Load Balancing, georedundante Standorte). Nachweis: Architektur-Diagramm, SLA-Dokumentation.",
			"Business Continuity", "automated", 2),

		// Art. 21(2)(d) — Supply Chain Security
		c("NIS2-D.1", "Lieferanten-Sicherheitsrichtlinie",
			"Definiere Mindest-Sicherheitsanforderungen für alle IKT-Lieferanten und Dienstleister. Nachweis: Lieferanten-Sicherheitsrichtlinie.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.2", "Lieferanten-Risikobewertung",
			"Bewerte das Sicherheitsrisiko aller wesentlichen Lieferanten vor Vertragsabschluss und danach jährlich. Nachweis: Lieferanten-Risikobewertungsberichte.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.3", "Sicherheitsanforderungen in Verträgen",
			"Verankere verbindliche Sicherheitsanforderungen (DSGVO-AVV, ISO 27001, Auditrechte) in allen IKT-Verträgen. Nachweis: Vertragsklauseln, AVV-Mustervorlage.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.4", "Zugriffsverwaltung für Drittparteien",
			"Steuere und überwache Remote-Zugriffe von Lieferanten und externen Dienstleistern. Nachweis: Zugriffskonzept, Protokolle externer Zugriffe.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.5", "Software-Lieferkettensicherheit",
			"Prüfe eingesetzte Open-Source- und Third-Party-Software auf bekannte Schwachstellen (SBOM, Dependency-Scanning). Nachweis: SBOM, Scanner-Berichte.",
			"Supply Chain", "manual", 3),
		c("NIS2-D.6", "Sicherheitsprüfung von IKT-Produkten",
			"Führe vor dem Einsatz neuer IKT-Produkte eine Sicherheitsprüfung durch (Zertifizierungen, Herstellernachweise). Nachweis: Produktprüfungs-Checkliste.",
			"Supply Chain", "manual", 2),
		c("NIS2-D.7", "Lieferanten-Monitoring",
			"Überwache laufend Sicherheitsmeldungen und Statusänderungen kritischer Lieferanten. Nachweis: Monitoring-Prozess, Eskalationsverfahren.",
			"Supply Chain", "manual", 1),
		c("NIS2-D.8", "Subunternehmer- und Outsourcing-Management",
			"Stelle sicher, dass Sicherheitsanforderungen bei Weitervergabe an Subunternehmer gewahrt bleiben. Nachweis: Outsourcing-Richtlinie, Vertragsklauseln.",
			"Supply Chain", "manual", 1),

		// Art. 21(2)(e) — Netz- und IS-Sicherheit
		c("NIS2-E.1", "Sicherer Entwicklungszyklus (SDLC)",
			"Integriere Sicherheitsanforderungen in alle Phasen des Softwareentwicklungsprozesses (Threat Modeling, Code Review, Security Testing). Nachweis: SDLC-Dokumentation, Review-Nachweise.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.2", "Sicherheitsanforderungen bei Systembeschaffung",
			"Definiere und prüfe Sicherheitsanforderungen vor Beschaffung neuer IT-Systeme. Nachweis: Beschaffungs-Checkliste mit Sicherheitskriterien.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.3", "Schwachstellenmanagement-Programm",
			"Betreibe ein strukturiertes Programm zur Identifikation, Bewertung und Behebung technischer Schwachstellen. Nachweis: Scanner-Berichte, Ticket-System-Auszüge.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.4", "Patch-Management",
			"Stelle sicher, dass Sicherheits-Patches für kritische Systeme innerhalb definierter Fristen eingespielt werden (kritisch: ≤72 h). Nachweis: Patch-Berichte, SLA-Dokumentation.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.5", "Penetrationstests",
			"Führe mindestens jährlich Penetrationstests durch kritische Systeme durch. Nachweis: Pentest-Berichte mit Datum, Scope und Ergebnissen.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.6", "Responsible Vulnerability Disclosure",
			"Etabliere einen Prozess zur Entgegennahme und Bearbeitung extern gemeldeter Schwachstellen. Nachweis: Responsible-Disclosure-Policy (z.B. security.txt).",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.7", "Änderungsmanagement (Change Management)",
			"Stelle sicher, dass alle Änderungen an IT-Systemen genehmigt, getestet und dokumentiert werden. Nachweis: Change-Management-Prozess, Genehmigungsnachweise.",
			"Netz- & IS-Sicherheit", "manual", 2),
		c("NIS2-E.8", "Netzarchitektur und Segmentierung",
			"Segmentiere das Netzwerk nach Schutzbedarf (DMZ, Produktions- vs. Entwicklungsnetz, OT-Trennung). Nachweis: Netzplan, Firewall-Regeln.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.9", "Firewall und Perimetersicherheit",
			"Betreibe Firewalls an allen Netzübergängen nach dem Least-Privilege-Prinzip. Überprüfe Regeln mindestens jährlich. Nachweis: Firewall-Konfiguration, Regelreviews.",
			"Netz- & IS-Sicherheit", "automated", 3),
		c("NIS2-E.10", "Einbruchserkennung und -prävention (IDS/IPS)",
			"Setze IDS/IPS-Systeme an kritischen Netzpunkten ein und stelle sicher, dass Alarme zeitnah bearbeitet werden. Nachweis: IDS/IPS-Konfiguration, Alarmierungsprotokoll.",
			"Netz- & IS-Sicherheit", "automated", 2),
		c("NIS2-E.11", "Sichere Konfigurationsverwaltung",
			"Nutze Hardening-Leitlinien (CIS Benchmarks, BSI SiM) für alle eingesetzten Systeme. Nachweis: Konfigurationsbaselines, Compliance-Scan-Berichte.",
			"Netz- & IS-Sicherheit", "automated", 2),

		// Art. 21(2)(f) — Wirksamkeitsbewertung
		c("NIS2-F.1", "Cybersicherheits-KPIs und Metriken",
			"Definiere messbare KPIs für die Sicherheitsleistung (z.B. MTTR, offene Schwachstellen, Patch-Compliance-Rate). Nachweis: KPI-Definition, monatliche Berichte.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.2", "Internes Sicherheitsauditprogramm",
			"Führe mindestens jährlich interne IS-Audits durch und dokumentiere Befunde und Maßnahmen. Nachweis: Auditplan, Auditberichte.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.3", "Management-Review der Sicherheitsleistung",
			"Halte mindestens jährlich ein Management-Review der IS-Leistung ab. Nachweis: Meeting-Protokolle, Entscheidungsdokumentation.",
			"Wirksamkeitsbewertung", "manual", 2),
		c("NIS2-F.4", "Kontinuierlicher Verbesserungsprozess",
			"Etabliere einen formalen KVP, der Erkenntnisse aus Audits, Vorfällen und Reviews in konkrete Verbesserungen überführt. Nachweis: Maßnahmenverfolgung (z.B. Ticketsystem).",
			"Wirksamkeitsbewertung", "manual", 1),
		c("NIS2-F.5", "Externe Zertifizierung und Auditierung",
			"Plane externe Audits oder Zertifizierungen (z.B. ISO 27001) als Nachweis gegenüber Kunden und Behörden. Nachweis: Zertifikat, Auditbericht.",
			"Wirksamkeitsbewertung", "manual", 1),

		// Art. 21(2)(g) — Cyber-Hygiene und Schulungen
		c("NIS2-G.1", "Cybersicherheits-Awareness-Programm",
			"Betreibe ein dauerhaftes Awareness-Programm (Newsletter, Intranet, Poster) zur Sensibilisierung aller Mitarbeitenden. Nachweis: Programmbeschreibung, Materialien.",
			"Cyber-Hygiene & Training", "manual", 2),
		c("NIS2-G.2", "Sicherheitsschulung für alle Mitarbeitenden",
			"Schule alle Mitarbeitenden mindestens jährlich zu grundlegenden Sicherheitsthemen (Phishing, Passwortsicherheit, Datenschutz). Nachweis: Schulungsnachweise, Teilnehmerlisten.",
			"Cyber-Hygiene & Training", "manual", 3),
		c("NIS2-G.3", "Rollenbasierte Sicherheitsschulung",
			"Biete zusätzliche Schulungen für sicherheitskritische Rollen an (Admins, Entwickler, Management). Nachweis: rollenspezifische Schulungspläne und Teilnahmenachweise.",
			"Cyber-Hygiene & Training", "manual", 2),
		c("NIS2-G.4", "Phishing-Simulationen",
			"Führe regelmäßige (mind. 2x/Jahr) Phishing-Simulationen durch und nutze Ergebnisse für gezielte Nachschulung. Nachweis: Simulationsberichte mit Klickraten und Folgemaßnahmen.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.5", "Passwort- und Authentifizierungsrichtlinie",
			"Lege Mindestanforderungen für Passwörter und Authentifizierung fest (Länge, Komplexität, Wiederverwendung, Passwortmanager). Nachweis: Richtliniendokument, technische Durchsetzung.",
			"Cyber-Hygiene & Training", "manual", 3),
		c("NIS2-G.6", "E-Mail-Sicherheitskontrollen",
			"Implementiere E-Mail-Sicherheitsmaßnahmen (SPF, DKIM, DMARC, Anti-Spam, Anti-Phishing). Nachweis: DNS-Einträge, E-Mail-Gateway-Konfiguration.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.7", "Malware-Schutz und Antivirus",
			"Setze Endpoint-Protection-Software auf allen Endgeräten ein und stelle automatische Signatur-Updates sicher. Nachweis: AV-Konfiguration, Scan-Berichte.",
			"Cyber-Hygiene & Training", "automated", 3),
		c("NIS2-G.8", "Endpoint Detection and Response (EDR)",
			"Implementiere EDR-Software zur verhaltensbasierten Erkennung von Angriffen auf Endgeräten. Nachweis: EDR-Konfiguration, Alarmierungsprotokoll.",
			"Cyber-Hygiene & Training", "automated", 2),
		c("NIS2-G.9", "Web-Filterung und DNS-Sicherheit",
			"Setze Web-Proxy oder DNS-Filtering ein, um den Aufruf schädlicher Websites zu verhindern. Nachweis: Filterlisten-Konfiguration, DNS-Sicherheitsberichte.",
			"Cyber-Hygiene & Training", "automated", 2),

		// Art. 21(2)(h) — Kryptographie
		c("NIS2-H.1", "Kryptographierichtlinie",
			"Erstelle eine Richtlinie zu zulässigen kryptographischen Verfahren und deren Einsatzgebieten. Nachweis: genehmigtes Richtliniendokument.",
			"Kryptographie", "manual", 2),
		c("NIS2-H.2", "Schlüsselverwaltungsverfahren",
			"Dokumentiere den gesamten Lebenszyklus kryptographischer Schlüssel (Generierung, Verteilung, Speicherung, Widerruf, Vernichtung). Nachweis: Schlüsselverwaltungsverfahren, KMS-Konfiguration.",
			"Kryptographie", "manual", 2),
		c("NIS2-H.3", "Verschlüsselung ruhender Daten",
			"Verschlüssele alle sensiblen Daten in Ruhe (Datenbanken, Backups, Dateisysteme) mit aktuellen Verfahren (AES-256). Nachweis: Verschlüsselungskonfiguration, Scanner-Berichte.",
			"Kryptographie", "automated", 3),
		c("NIS2-H.4", "Verschlüsselung übertragener Daten (TLS)",
			"Stelle sicher, dass alle Datenübertragungen verschlüsselt erfolgen (TLS 1.2+, keine veralteten Protokolle). Nachweis: TLS-Scan-Bericht (z.B. SSL Labs), Konfigurationsdokumentation.",
			"Kryptographie", "automated", 3),
		c("NIS2-H.5", "Zertifikats-Lifecycle-Management",
			"Verwalte alle TLS/SSL-Zertifikate zentral, überwache Ablaufdaten und erneuere rechtzeitig. Nachweis: Zertifikatsregister, Erneuerungsprozess.",
			"Kryptographie", "automated", 2),
		c("NIS2-H.6", "Zulässige kryptographische Algorithmen",
			"Führe eine Liste genehmigter Algorithmen und Schlüssellängen (BSI TR-02102) und schließe veraltete Verfahren (MD5, SHA-1, DES) aus. Nachweis: Algorithmenliste, Konfigurationsprüfung.",
			"Kryptographie", "manual", 1),

		// Art. 21(2)(i) — HR-Sicherheit, Zugriffskontrolle, Asset-Management
		c("NIS2-I.1", "HR-Sicherheitsrichtlinie",
			"Definiere Sicherheitsanforderungen für alle Phasen des Beschäftigungsverhältnisses (Einstellung, laufend, Austritt). Nachweis: HR-Sicherheitsrichtlinie.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.2", "Hintergrundüberprüfungen (Screening)",
			"Führe bei Einstellung und für sicherheitskritische Rollen Hintergrundüberprüfungen durch (soweit gesetzlich zulässig). Nachweis: Screening-Richtlinie, Nachweisarchivierung.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.3", "Richtlinie zur akzeptablen Nutzung",
			"Kommuniziere eine verbindliche Richtlinie zur akzeptablen Nutzung von IT-Ressourcen an alle Mitarbeitenden. Nachweis: Richtlinie, Unterschriften/Bestätigungen der Mitarbeitenden.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.4", "Offboarding- und Kündigungsverfahren",
			"Stelle sicher, dass beim Austritt alle Zugänge zeitnah gesperrt, Assets zurückgegeben und Wissenstransfer sichergestellt wird. Nachweis: Offboarding-Checkliste.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.5", "Zugriffskontrollrichtlinie",
			"Definiere das Prinzip der minimalen Rechtevergabe und dokumentiere den Genehmigungsprozess für Zugriffsrechte. Nachweis: Zugriffskontrollrichtlinie.",
			"Zugang & Identität", "manual", 3),
		c("NIS2-I.6", "Identity- und Access-Management (IAM)",
			"Betreibe ein zentrales IAM-System für die Verwaltung aller Benutzerkonten und -rechte. Nachweis: IAM-Systemdokumentation, Provisionierungsprozess.",
			"Zugang & Identität", "automated", 3),
		c("NIS2-I.7", "Privileged Access Management (PAM)",
			"Verwalte privilegierte Konten (Admins, Root) gesondert mit PAM-Lösung, Vier-Augen-Prinzip und vollständigem Logging. Nachweis: PAM-Konfiguration, Zugriffsprotokoll.",
			"Zugang & Identität", "automated", 3),
		c("NIS2-I.8", "Rollenbasierte Zugriffssteuerung (RBAC)",
			"Implementiere rollenbasierte Berechtigungskonzepte für alle kritischen Systeme. Nachweis: Rollenmatrix, Berechtigungskonzept.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.9", "Regelmäßige Zugriffsüberprüfungen",
			"Überprüfe mindestens halbjährlich alle vergebenen Zugriffsrechte auf Aktualität und Notwendigkeit. Nachweis: Prüfprotokolle, Bereinigungsnachweise.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.10", "Physische Sicherheitsmaßnahmen",
			"Sichere Serverräume, Büros und Arbeitsplätze physisch gegen unbefugten Zugang (Zutrittskontrolle, CCTV, Clean-Desk). Nachweis: Zutrittskontrollkonzept, Begehungsprotokoll.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.11", "Asset-Erfassung, -Kennzeichnung und -Entsorgung",
			"Kennzeichne alle Hardware-Assets, erfasse sie im Inventar und stelle datensichere Entsorgung sicher (z.B. DSGVO-konformes Löschen). Nachweis: Asset-Register, Entsorgungsnachweise.",
			"Zugang & Identität", "manual", 1),
		c("NIS2-I.12", "Mobile-Device- und BYOD-Management",
			"Verwalte Mobilgeräte über MDM-Lösung, setze Geräteverschlüsselung und Remote-Wipe durch. Nachweis: MDM-Konfiguration, BYOD-Richtlinie.",
			"Zugang & Identität", "manual", 2),
		c("NIS2-I.13", "Logging, Monitoring und SIEM",
			"Protokolliere sicherheitsrelevante Ereignisse auf allen kritischen Systemen und überwache zentral via SIEM. Nachweis: Log-Konfiguration, SIEM-Architektur, Aufbewahrungsrichtlinie.",
			"Zugang & Identität", "automated", 3),

		// Art. 21(2)(j) — MFA und sichere Kommunikation
		c("NIS2-J.1", "Multi-Faktor-Authentifizierung (MFA)",
			"Erzwinge MFA für alle Benutzer bei Zugriff auf Unternehmensanwendungen und -systeme. Nachweis: MFA-Konfiguration, Ausnahmeliste mit Begründungen.",
			"Authentifizierung & Kommunikation", "automated", 3),
		c("NIS2-J.2", "MFA für privilegierte und Remote-Konten",
			"Stelle sicher, dass Administratoren und Remote-Nutzer ausnahmslos MFA verwenden. Nachweis: PAM-Konfiguration, VPN-Zugangsprotokolle.",
			"Authentifizierung & Kommunikation", "automated", 3),
		c("NIS2-J.3", "Richtlinie für Remote-Zugang",
			"Definiere zulässige Methoden und Anforderungen für Remote-Zugang (VPN, Zero Trust, MFA, Gerätezertifikate). Nachweis: Remote-Access-Richtlinie.",
			"Authentifizierung & Kommunikation", "manual", 3),
		c("NIS2-J.4", "VPN und sicherer Remote-Zugang",
			"Setze ein verschlüsseltes VPN oder Zero-Trust-Netzwerkzugangslösung für alle Remote-Verbindungen ein. Nachweis: VPN-Konfiguration, Zertifikatsdokumentation.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.5", "Verschlüsselte Kommunikation (Sprache, Video, Text)",
			"Nutze ausschließlich verschlüsselte Kommunikationstools für dienstliche Kommunikation (Signal, Teams mit E2E, etc.). Nachweis: Tool-Richtlinie, Konfiguration.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.6", "Endpunktsicherheit für Remote-Zugang",
			"Stelle sicher, dass Remote-Endgeräte Sicherheitsanforderungen erfüllen (Verschlüsselung, aktuelle AV, MDM). Nachweis: Endpoint-Compliance-Berichte.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.7", "Mobile-Device-Sicherheit",
			"Konfiguriere mobile Geräte mit Bildschirmsperre, Verschlüsselung und Remote-Wipe-Fähigkeit. Nachweis: MDM-Konfiguration, Compliance-Bericht.",
			"Authentifizierung & Kommunikation", "automated", 2),
		c("NIS2-J.8", "Notfallkommunikationssysteme",
			"Halte Notfallkommunikationsmittel bereit, die unabhängig von der normalen IT-Infrastruktur funktionieren (Satelliten-Telefon, Out-of-Band-Kommunikation). Nachweis: Inventarliste, Testprotokoll.",
			"Authentifizierung & Kommunikation", "manual", 1),
	}
}

func iso27001Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// A.5 — Informationssicherheitsrichtlinien
		c("A.5.1", "Richtlinien zur Informationssicherheit", "Definiere den Rahmen für alle IS-Richtlinien der Organisation.", "Richtlinien", "manual", 2),
		c("A.5.1.1", "Richtlinien für Informationssicherheit", "Erstelle ein vollständiges Set genehmigter IS-Richtlinien. Nachweis: aktuelle, unterschriebene Richtliniendokumente.", "Richtlinien", "manual", 2),
		c("A.5.1.2", "Überprüfung der Richtlinien für Informationssicherheit", "Überprüfe alle Richtlinien mindestens jährlich. Nachweis: Revisionshistorie mit Datum und Genehmigung.", "Richtlinien", "manual", 1),

		// A.6 — Organisation der Informationssicherheit
		c("A.6.1", "Interne Organisation", "Stelle sicher, dass IS-Verantwortlichkeiten klar geregelt sind.", "Organisation", "manual", 2),
		c("A.6.1.1", "Rollen und Verantwortlichkeiten für Informationssicherheit", "Weise IS-Rollen (ISB, Datenschutzbeauftragter, etc.) explizit zu. Nachweis: Stellenbeschreibungen, Beauftragungsschreiben.", "Organisation", "manual", 2),
		c("A.6.1.2", "Aufgabentrennung", "Trenne unvereinbare Aufgaben (z.B. Entwicklung/Freigabe). Nachweis: Rollenmatrix mit Trennungsnachweis.", "Organisation", "manual", 1),
		c("A.6.1.3", "Kontakt mit Behörden", "Pflege aktuelle Kontaktinformationen zu relevanten Behörden (BSI, Datenschutzbehörden). Nachweis: Kontaktliste.", "Organisation", "manual", 1),
		c("A.6.1.5", "Informationssicherheit im Projektmanagement", "Integriere IS-Anforderungen in alle Projektprozesse. Nachweis: Projektcheckliste mit IS-Punkten.", "Organisation", "manual", 1),
		c("A.6.2", "Mobilgeräte und Telearbeit", "Manage Risiken durch mobile Geräte und Heimarbeit.", "Organisation", "manual", 2),
		c("A.6.2.1", "Richtlinie für mobile Geräte", "Definiere zulässige Nutzung und Sicherheitsanforderungen für mobile Geräte. Nachweis: MDM-Konfiguration, Richtliniendokument.", "Organisation", "manual", 2),
		c("A.6.2.2", "Telearbeit", "Stelle sichere Arbeitsmöglichkeiten für Heimarbeitsplätze sicher. Nachweis: Telearbeitsrichtlinie, VPN-Konfiguration.", "Organisation", "manual", 1),

		// A.8 — Asset Management
		c("A.8.1", "Verantwortung für Assets", "Inventarisiere und klassifiziere alle Informationsassets.", "Asset Management", "automated", 2),
		c("A.8.1.1", "Inventarisierung von Assets", "Führe ein vollständiges, aktuelles Asset-Register. Nachweis: Asset-Inventar mit letztem Aktualisierungsdatum.", "Asset Management", "automated", 2),
		c("A.8.1.2", "Eigentümerschaft von Assets", "Weise jedem Asset einen verantwortlichen Eigentümer zu. Nachweis: Asset-Register mit Eigentümerfeld.", "Asset Management", "manual", 1),
		c("A.8.1.3", "Zulässige Nutzung von Assets", "Dokumentiere akzeptable Nutzungsregeln für alle Asset-Klassen. Nachweis: Acceptable-Use-Policy.", "Asset Management", "manual", 1),
		c("A.8.1.4", "Rückgabe von Assets", "Stelle Rückgabe aller Assets bei Beschäftigungsende sicher. Nachweis: Offboarding-Checkliste.", "Asset Management", "manual", 1),
		c("A.8.2", "Klassifizierung von Informationen", "Klassifiziere alle Informationsassets nach Vertraulichkeit, Integrität und Verfügbarkeit. Nachweis: Klassifizierungsschema, Asset-Register mit Klassifizierungsfeld.", "Asset Management", "manual", 2),
		c("A.8.3", "Handhabung von Datenträgern", "Manage physische und digitale Datenträger sicher über ihren gesamten Lebenszyklus (Einsatz, Transport, Entsorgung). Nachweis: Datenträger-Policy, Vernichtungsnachweise.", "Asset Management", "manual", 2),

		// A.9 — Zugangskontrolle
		c("A.9.1", "Geschäftsanforderungen an die Zugangskontrolle", "Definiere Zugangskontrollrichtlinie basierend auf Geschäftsbedarf.", "Zugangskontrolle", "automated", 3),
		c("A.9.1.1", "Zugangssteuerungsrichtlinie", "Erstelle eine schriftliche Zugangskontrollrichtlinie (Need-to-know, Least Privilege). Nachweis: genehmigtes Dokument.", "Zugangskontrolle", "manual", 3),
		c("A.9.1.2", "Zugang zu Netzwerken und Netzwerkdiensten", "Beschränke Netzwerkzugänge auf autorisierte Nutzer und Geräte. Nachweis: NAC-Konfiguration, Firewall-Regeln.", "Zugangskontrolle", "automated", 2),
		c("A.9.2", "Benutzerzugangsverwaltung", "Manage Benutzerkonten über den gesamten Lebenszyklus.", "Zugangskontrolle", "automated", 3),
		c("A.9.2.1", "Registrierung und Deregistrierung von Benutzern", "Formalisiere Onboarding/Offboarding-Prozesse für Konten. Nachweis: Provisionierungs-Workflow.", "Zugangskontrolle", "automated", 2),
		c("A.9.2.2", "Benutzerzugangsprovisionierung", "Stelle sicher, dass Zugriffsrechte nur nach Genehmigung erteilt werden. Nachweis: Genehmigungsprotokoll.", "Zugangskontrolle", "automated", 2),
		c("A.9.2.3", "Verwaltung privilegierter Zugriffsrechte", "Verwalte Admin-Rechte restriktiv mit PAM-Lösung. Nachweis: PAM-Konfiguration, Zugriffsprotokoll.", "Zugangskontrolle", "automated", 3),
		c("A.9.2.5", "Überprüfung von Benutzerzugriffsrechten", "Überprüfe Zugriffsrechte halbjährlich auf Aktualität. Nachweis: Review-Protokolle.", "Zugangskontrolle", "manual", 2),
		c("A.9.4", "Zugangs- und Passwortverwaltung", "Sichere Systemzugänge durch technische Maßnahmen.", "Zugangskontrolle", "automated", 3),
		c("A.9.4.1", "Zugang zu Informationen einschränken", "Setze Least-Privilege auf Applikationsebene durch. Nachweis: Berechtigungskonzept.", "Zugangskontrolle", "automated", 2),
		c("A.9.4.2", "Sichere Anmeldeverfahren", "Erzwinge MFA und sichere Login-Mechanismen. Nachweis: MFA-Konfiguration.", "Zugangskontrolle", "automated", 3),
		c("A.9.4.3", "Passwortverwaltungssystem", "Setze einen Passwort-Manager oder Single-Sign-On ein. Nachweis: Tool-Konfiguration, Richtlinie.", "Zugangskontrolle", "automated", 3),

		// A.10 — Kryptographie
		c("A.10.1", "Kryptographische Maßnahmen", "Stelle den richtigen Einsatz von Kryptographie sicher.", "Kryptographie", "manual", 2),
		c("A.10.1.1", "Richtlinie für den Einsatz kryptographischer Maßnahmen", "Definiere zulässige Algorithmen, Schlüssellängen und Einsatzgebiete. Nachweis: Kryptographierichtlinie.", "Kryptographie", "manual", 2),
		c("A.10.1.2", "Schlüsselverwaltung", "Dokumentiere Schlüssellebenszyklus (Generierung, Verteilung, Widerruf). Nachweis: Key-Management-Prozess, KMS-Konfiguration.", "Kryptographie", "manual", 2),

		// A.12 — Betrieb
		c("A.12.1", "Betriebsverfahren und Verantwortlichkeiten", "Dokumentiere und manage IT-Betriebsprozesse.", "Betrieb", "manual", 2),
		c("A.12.1.1", "Dokumentierte Betriebsverfahren", "Erstelle schriftliche Betriebshandbücher für alle kritischen Systeme. Nachweis: Betriebsdokumentation.", "Betrieb", "manual", 2),
		c("A.12.1.2", "Änderungsmanagement", "Stelle sicher, dass alle IT-Änderungen geplant, genehmigt und dokumentiert werden. Nachweis: Change-Tickets.", "Betrieb", "manual", 2),
		c("A.12.2", "Schutz vor Schadsoftware", "Schütze Systeme vor Malware durch präventive und erkennende Maßnahmen.", "Betrieb", "automated", 3),
		c("A.12.3", "Datensicherung", "Stelle Datenverfügbarkeit durch regelmäßige Backups sicher.", "Betrieb", "automated", 3),
		c("A.12.3.1", "Sicherung von Informationen", "Implementiere automatisierte Backups nach 3-2-1-Prinzip. Nachweis: Backup-Job-Konfiguration, Testberichte.", "Betrieb", "automated", 3),
		c("A.12.4", "Protokollierung und Überwachung", "Protokolliere sicherheitsrelevante Ereignisse und überwache Systeme auf Anomalien. Nachweis: SIEM-Konfiguration, Log-Aufbewahrungsrichtlinie, Alerting-Regelwerk.", "Betrieb", "automated", 3),
		c("A.12.6", "Management technischer Schwachstellen", "Reduziere Angriffsfläche durch zeitnahes Schwachstellenmanagement.", "Betrieb", "automated", 3),
		c("A.12.6.1", "Management technischer Schwachstellen", "Scanne regelmäßig auf Schwachstellen und behebe kritische innerhalb definierter Fristen. Nachweis: Scanner-Berichte, Patch-Protokoll.", "Betrieb", "automated", 3),

		// A.14 — Systembeschaffung, -entwicklung und -wartung
		c("A.14.1", "Sicherheitsanforderungen an Informationssysteme", "Definiere IS-Anforderungen bei Beschaffung und Entwicklung.", "Systementwicklung", "manual", 2),
		c("A.14.1.1", "Analyse und Spezifikation von Anforderungen an die Informationssicherheit", "Dokumentiere IS-Anforderungen in Pflichtenheften. Nachweis: Anforderungsdokument.", "Systementwicklung", "manual", 2),
		c("A.14.1.2", "Absicherung von Anwendungsdiensten in öffentlichen Netzen", "Sichere Web-Dienste gegen OWASP Top 10. Nachweis: Pentest-Bericht, WAF-Konfiguration.", "Systementwicklung", "manual", 2),
		c("A.14.2", "Sicherheit in Entwicklungs- und Unterstützungsprozessen", "Integriere Sicherheit in den gesamten SDLC.", "Systementwicklung", "manual", 2),
		c("A.14.2.1", "Richtlinie zur sicheren Entwicklung", "Erstelle und kommuniziere Secure-Coding-Richtlinien. Nachweis: Richtliniendokument, Schulungsnachweise.", "Systementwicklung", "manual", 2),
		c("A.14.2.8", "Testen der Systemsicherheit", "Führe Sicherheitstests (SAST, DAST, Pentest) vor Releases durch. Nachweis: Testberichte.", "Systementwicklung", "manual", 2),

		// A.16 — Handhabung von Informationssicherheitsvorfällen
		c("A.16.1", "Management von Informationssicherheitsvorfällen", "Etabliere einen strukturierten Prozess zur Vorfallbehandlung.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.1", "Verantwortlichkeiten und Verfahren", "Definiere klare Rollen und Abläufe für Vorfallreaktionen. Nachweis: IR-Plan, Teambesetzungsplan.", "Vorfallmanagement", "manual", 2),
		c("A.16.1.2", "Meldung von Informationssicherheitsereignissen", "Etabliere einfache Meldekanäle für alle Mitarbeitenden. Nachweis: Meldeprozess, Kontaktinfos.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.4", "Bewertung von und Entscheidung über Informationssicherheitsereignisse", "Stelle sicher, dass Ereignisse zeitnah klassifiziert werden. Nachweis: Klassifizierungsmatrix.", "Vorfallmanagement", "manual", 2),
		c("A.16.1.5", "Reaktion auf Informationssicherheitsvorfälle", "Definiere konkrete Reaktionsschritte je Vorfallklasse. Nachweis: IR-Playbooks.", "Vorfallmanagement", "manual", 3),
		c("A.16.1.6", "Erkenntnisse aus Informationssicherheitsvorfällen", "Führe Post-Incident-Reviews durch und leite Verbesserungen ab. Nachweis: Review-Berichte.", "Vorfallmanagement", "manual", 2),

		// A.17 — Business Continuity
		c("A.17.1", "Kontinuität der Informationssicherheit", "Stelle IS-Kontinuität im Krisenfall sicher.", "Business Continuity", "manual", 2),
		c("A.17.1.1", "Planung der Kontinuität der Informationssicherheit", "Erstelle BCM-Plan mit IS-Komponente. Nachweis: BCM-Plan-Dokument.", "Business Continuity", "manual", 2),
		c("A.17.1.2", "Implementierung der Kontinuität der Informationssicherheit", "Setze BCM-Maßnahmen technisch und organisatorisch um. Nachweis: Implementierungsnachweis.", "Business Continuity", "manual", 2),
		c("A.17.1.3", "Überprüfung, Überarbeitung und Bewertung der Kontinuität der Informationssicherheit", "Teste und überprüfe BCM-Pläne mindestens jährlich. Nachweis: Testberichte.", "Business Continuity", "manual", 1),

		// A.18 — Compliance
		c("A.18.1", "Einhaltung gesetzlicher und vertraglicher Anforderungen", "Identifiziere und erfülle alle anwendbaren Rechtspflichten.", "Compliance", "third_party", 2),
		c("A.18.1.1", "Identifizierung anwendbarer Gesetze und vertraglicher Anforderungen", "Pflege ein Compliance-Register aller relevanten Gesetze und Verträge. Nachweis: Compliance-Register.", "Compliance", "manual", 2),
		c("A.18.1.3", "Schutz von Aufzeichnungen", "Stelle Aufbewahrung und Schutz von Aufzeichnungen gem. gesetzlicher Fristen sicher. Nachweis: Aufbewahrungsrichtlinie.", "Compliance", "manual", 1),
		c("A.18.1.4", "Datenschutz und Schutz von personenbezogenen Daten", "Stelle DSGVO-Konformität sicher. Nachweis: Verzeichnis der Verarbeitungstätigkeiten, DSFA.", "Compliance", "manual", 3),
		c("A.18.2", "Überprüfung der Informationssicherheit", "Prüfe regelmäßig die Einhaltung der IS-Vorgaben.", "Compliance", "manual", 2),
		c("A.18.2.2", "Einhaltung von Sicherheitsrichtlinien und -standards", "Überprüfe technische Systeme auf Konformität mit IS-Richtlinien. Nachweis: Compliance-Scan-Berichte.", "Compliance", "manual", 2),

		// A.7 — Personalsicherheit (ISO 27001:2013; in 2022: Kap. 6)
		c("A.7.1.1", "Überprüfung von Bewerbern", "Führe Hintergrundprüfungen (Zeugnisse, Referenzen, ggf. polizeiliches Führungszeugnis) vor der Einstellung durch. Nachweis: Screening-Richtlinie, Nachweisdokumentation.", "Personalsicherheit", "manual", 2),
		c("A.7.1.2", "Beschäftigungsbedingungen", "Verpflichte Mitarbeitende vertraglich auf Informationssicherheits- und Datenschutzpflichten. Nachweis: Muster-Arbeitsvertrag mit IS-Klauseln, unterzeichnete Vertraulichkeitserklärungen.", "Personalsicherheit", "manual", 2),
		c("A.7.2", "Während der Beschäftigung", "Stelle sicher, dass Mitarbeitende IS-Pflichten kennen und erfüllen. Nachweis: Schulungsnachweise, Disziplinarrichtlinie.", "Personalsicherheit", "manual", 2),
		c("A.7.2.1", "Verantwortlichkeiten der Leitung", "Stelle sicher, dass die Leitung IS-Pflichten kommuniziert, Schulungen fördert und Einhaltung einfordert. Nachweis: Management-Richtlinie, Kommunikationsnachweise.", "Personalsicherheit", "manual", 2),
		c("A.7.2.2", "IS-Bewusstsein, -Ausbildung und -Schulung", "Schule alle Mitarbeitenden mindestens jährlich zu IS-Grundlagen, aktuellen Bedrohungen und internen Richtlinien. Nachweis: Schulungsnachweise, Teilnehmerlisten, Schulungsplan.", "Personalsicherheit", "manual", 3),
		c("A.7.2.3", "Disziplinarverfahren", "Definiere und kommuniziere Konsequenzen bei Verstößen gegen IS-Richtlinien. Nachweis: HR-Richtlinie, Kommunikationsnachweis.", "Personalsicherheit", "manual", 1),
		c("A.7.3.1", "Pflichten bei Beendigung des Arbeitsverhältnisses", "Stelle bei Austritt sicher, dass alle Zugänge gesperrt, Assets zurückgegeben und Vertraulichkeitspflichten kommuniziert werden. Nachweis: Offboarding-Checkliste, Nachweise.", "Personalsicherheit", "manual", 2),

		// A.11 — Physische Sicherheit (ISO 27001:2013; in 2022: Kap. 7)
		c("A.11.1.1", "Physischer Sicherheitsperimeter", "Definiere Sicherheitsbereiche (Serverräume, Büros, RZ) und sichere sie mit physischen Barrieren und Zutrittskontrollen. Nachweis: Raumkonzept, Zutrittskontroll-Dokumentation.", "Physische Sicherheit", "manual", 3),
		c("A.11.1.2", "Physische Zutrittssteuerung", "Implementiere elektronische Zutrittskontrolle mit individueller Authentifizierung, Protokollierung und Besuchermanagement. Nachweis: Zutrittssystem, Zugangsprotokolle.", "Physische Sicherheit", "manual", 3),
		c("A.11.1.3", "Sicherung von Büros und Einrichtungen", "Sichere Büros und Nebenräume physisch: abschließbare Schränke, keine sensitiven Informationen offen sichtbar (Clean Desk). Nachweis: Begehungsprotokoll, Hausordnung.", "Physische Sicherheit", "manual", 2),
		c("A.11.1.5", "Arbeiten in sicheren Bereichen", "Definiere Verhaltensregeln in sicherheitskritischen Bereichen (kein Fotografieren, Besucher begleiten, Bildschirmsperre). Nachweis: Richtlinie, Einweisung.", "Physische Sicherheit", "manual", 1),
		c("A.11.2.1", "Aufstellung und Schutz von Betriebsmitteln", "Platziere Server und kritische Hardware in kontrollierten Umgebungen (Klimatisierung, unterbrechungsfreie Stromversorgung). Nachweis: RZ-Konzept, USV-Konfiguration.", "Physische Sicherheit", "manual", 2),
		c("A.11.2.5", "Entfernen von Betriebsmitteln", "Genehmige und protokolliere das Entfernen von Hardware aus Sicherheitsbereichen. Nachweis: Asset-Tracking, Genehmigungsprotokoll.", "Physische Sicherheit", "manual", 1),
		c("A.11.2.7", "Sichere Entsorgung von Betriebsmitteln", "Lösche Datenträger sicher (NIST 800-88, Degaussing, physische Vernichtung) vor Entsorgung. Nachweis: Vernichtungsnachweise, zertifizierter Dienstleister.", "Physische Sicherheit", "manual", 3),

		// A.13 — Kommunikationssicherheit (ISO 27001:2013; in 2022: Kap. 8)
		c("A.13.1", "Netzwerksicherheitsmanagement", "Schütze Informationen in Netzwerken und unterstützenden IT-Infrastrukturkomponenten. Nachweis: Netzwerksicherheitsrichtlinie, Netzwerkplan, Firewall-Konfiguration.", "Kommunikationssicherheit", "automated", 3),
		c("A.13.1.1", "Steuerungsmaßnahmen für Netze", "Betreibe Netzwerke unter Sicherheitsgesichtspunkten: Netzstruktur, Zugangskontrolle, Monitoring. Nachweis: Netzwerkplan, Firewall-Konfiguration.", "Kommunikationssicherheit", "automated", 3),
		c("A.13.1.2", "Sicherheit von Netzdiensten", "Definiere Sicherheitsanforderungen für alle genutzten Netzdienste (intern wie extern) und verankere sie in SLAs/AVVs. Nachweis: Dienstleistungsverträge, SLA-Anforderungen.", "Kommunikationssicherheit", "manual", 2),
		c("A.13.1.3", "Trennung in Netzen", "Segmentiere das Netzwerk nach Schutzbedarf (DMZ, Produktions-/Entwicklungsnetz, Gastnetze). Nachweis: Netzwerkplan mit Segmentierungskonzept, Firewall-Regeln.", "Kommunikationssicherheit", "automated", 3),
		c("A.13.2.1", "Richtlinien für Informationsübertragung", "Definiere Regeln für die sichere Übertragung von Informationen (verschlüsselt per E-Mail, gesicherter Datentransfer). Nachweis: Übertragungsrichtlinie.", "Kommunikationssicherheit", "manual", 2),
		c("A.13.2.4", "Vertraulichkeitsvereinbarungen", "Stelle sicher, dass Personen mit Zugang zu sensiblen Informationen aktuelle NDAs unterzeichnet haben. Nachweis: NDA-Muster, unterzeichnete Vereinbarungen.", "Kommunikationssicherheit", "manual", 2),

		// A.15 — Lieferantenbeziehungen (ISO 27001:2013; in 2022: Kap. 5)
		c("A.15.1.1", "IS-Richtlinie für Lieferantenbeziehungen", "Definiere Mindest-Sicherheitsanforderungen für alle Lieferanten und Dienstleister mit Zugang zu IS-relevanten Informationen oder Systemen. Nachweis: Lieferanten-Sicherheitsrichtlinie.", "Lieferantenbeziehungen", "manual", 2),
		c("A.15.1.2", "Sicherheitsaspekte in Lieferantenvereinbarungen", "Verankere IS-Anforderungen (DSGVO-AVV, Auditrechte, Vorfallmeldepflicht, Datenlöschung) verbindlich in allen Lieferantenverträgen. Nachweis: Vertragsklauseln, AVV-Muster.", "Lieferantenbeziehungen", "manual", 3),
		c("A.15.1.3", "IKT-Lieferkette", "Bewerte Sicherheitsrisiken in der IKT-Lieferkette (Software-Komponenten, Hardware-Hersteller, Cloud-Provider). Nachweis: Lieferketten-Risikobewertung, SBOM.", "Lieferantenbeziehungen", "manual", 2),
		c("A.15.2.1", "Überwachung von Lieferantendienstleistungen", "Überprüfe regelmäßig die IS-Leistung kritischer Lieferanten (Fragebögen, Zertifikate, Audits). Nachweis: Bewertungsberichte, Auditprotokolle.", "Lieferantenbeziehungen", "manual", 2),
		c("A.15.2.2", "Steuerung von Änderungen an Lieferantendienstleistungen", "Bewerte IS-Auswirkungen bei Änderungen von Lieferantendienstleistungen und steuere sie durch Change-Management. Nachweis: Change-Protokolle mit IS-Prüfung.", "Lieferantenbeziehungen", "manual", 1),

		// ISO 27001:2022 — 11 neue Controls (Annex A 2022-Nummerierung)
		c("A2022-5.7", "Threat Intelligence", "Sammle und analysiere Informationen zu Bedrohungen (CERT-Bund, CVE-Feeds, Threat-Intel-Plattformen) und integriere Erkenntnisse in den Risikoprozess. Nachweis: Threat-Intel-Quellen, Prozessdokumentation. (ISO 27001:2022 Annex A 5.7)", "Richtlinien", "manual", 2),
		c("A2022-5.23", "IS für Cloud-Dienste", "Definiere Sicherheitsanforderungen für alle genutzten Cloud-Dienste (IaaS, PaaS, SaaS) gemäß Schutzbedarf. Verifiziere Anbieter-Konformität (ISO 27001, SOC 2, BSI C5). Nachweis: Cloud-Sicherheitsrichtlinie, Anbieter-Zertifikate. (ISO 27001:2022 Annex A 5.23)", "Richtlinien", "third_party", 3),
		c("A2022-5.30", "IKT-Bereitschaft für Business Continuity", "Stelle sicher, dass IKT-Systeme für Betriebskontinuität ausgelegt sind (Redundanz, Failover, RTO/RPO). Teste mindestens jährlich. Nachweis: BCM-Plan mit IKT-Komponente, Testergebnisse. (ISO 27001:2022 Annex A 5.30)", "Business Continuity", "automated", 3),
		c("A2022-7.4", "Physische Sicherheitsüberwachung", "Überwache Sicherheitsbereiche durch Videoüberwachung, Einbruchsmeldeanlage oder Sicherheitspersonal. Nachweis: CCTV-Konzept, Alarmierungskonfiguration. (ISO 27001:2022 Annex A 7.4)", "Physische Sicherheit", "automated", 2),
		c("A2022-8.9", "Konfigurationsmanagement", "Definiere, implementiere und überwache sichere Konfigurationsbaselines für alle IT-Systeme (CIS Benchmarks). Nachweis: Hardening-Baseline, Konfigurationsscanning-Bericht. (ISO 27001:2022 Annex A 8.9)", "Betrieb", "automated", 3),
		c("A2022-8.10", "Informationslöschung", "Lösche Informationen sicher, wenn sie nicht mehr benötigt werden (NIST 800-88, sichere Formatierung, physische Vernichtung). Nachweis: Löschkonzept, Vernichtungsnachweise. (ISO 27001:2022 Annex A 8.10)", "Betrieb", "manual", 2),
		c("A2022-8.11", "Datenmaskierung", "Maskiere oder pseudonymisiere personenbezogene und sensible Daten in nicht-produktiven Umgebungen. Nachweis: Maskierungskonzept, technische Umsetzung in Test-/Staging-Systemen. (ISO 27001:2022 Annex A 8.11)", "Betrieb", "automated", 2),
		c("A2022-8.12", "Verhinderung von Datenlecks (DLP)", "Implementiere Data-Loss-Prevention-Maßnahmen für kritische Daten (E-Mail-DLP, Endpoint-DLP, Cloud-DLP). Nachweis: DLP-Konfiguration, Alerting-Protokoll. (ISO 27001:2022 Annex A 8.12)", "Betrieb", "automated", 2),
		c("A2022-8.16", "Überwachungsaktivitäten", "Überwache Systeme kontinuierlich auf Anomalien, ungewöhnliches Verhalten und Sicherheitsereignisse (SIEM, UEBA). Nachweis: SIEM-Use-Cases, Monitoring-Dashboard. (ISO 27001:2022 Annex A 8.16)", "Betrieb", "automated", 3),
		c("A2022-8.23", "Web-Filterung", "Nutze Web-Proxy oder DNS-Filterung, um Zugriff auf schädliche oder unerwünschte Websites zu verhindern. Nachweis: Filterkonfiguration, Kategorisierungsrichtlinie. (ISO 27001:2022 Annex A 8.23)", "Betrieb", "automated", 2),
		c("A2022-8.28", "Sichere Programmierung", "Wende sichere Programmierstandards an (OWASP Top 10, SANS CWE). Integriere SAST/DAST in CI/CD. Nachweis: Secure-Coding-Richtlinie, CI-Pipeline-Konfiguration, Scan-Berichte. (ISO 27001:2022 Annex A 8.28)", "Systementwicklung", "automated", 3),
	}
}

// bsiControls returns the BSI IT-Grundschutz-Kompendium baseline:
// 33 Bausteine über alle zehn Schichten (ISMS, ORP, CON, OPS, DER, APP,
// SYS, IND, NET, INF). Aufgebaut nach dem Pattern „Anforderung →
// Nachweis", analog zu craControls und doraControls.
//
// Schichten gem. BSI-Standard 200-2 / Kompendium 2023:
//
//	ISMS — Sicherheitsmanagement
//	ORP  — Organisation und Personal
//	CON  — Konzeption und Vorgehensweise
//	OPS  — Betrieb
//	DER  — Detektion und Reaktion
//	APP  — Anwendungen
//	SYS  — IT-Systeme
//	IND  — Industrielle IT (OT)
//	NET  — Netze und Kommunikation
//	INF  — Infrastruktur
//
// Diese 33 Bausteine bilden eine Basis-Absicherung gem. Standard 200-2 für
// kleine bis mittlere Organisationen ab. Ein Customer mit Kern-Absicherung
// erweitert sie um sektorale (IND.*) oder anwendungsspezifische (APP.*)
// Bausteine. Die Reihenfolge folgt dem Kompendium.
func bsiControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// ── ISMS: Sicherheitsmanagement (BSI-Standard 200-1/2) ──
		c("BSI-ISMS.1.A1", "Sicherheitsleitlinie",
			"Die Leitung verabschiedet eine schriftliche Informationssicherheitsleitlinie, in der Ziele, Geltungsbereich und Verantwortlichkeiten beschrieben sind. Mindestens jährlich überprüfen. Nachweis: unterzeichnete Leitlinie, Aktualisierungshistorie.",
			"Sicherheitsmanagement", "manual", 3),
		c("BSI-ISMS.1.A6", "Sicherheitskonzept",
			"Erstelle ein dokumentiertes Sicherheitskonzept gemäß BSI-Standard 200-2 (Basis- oder Kern-Absicherung). Es beschreibt den Geltungsbereich, die Schutzbedarfsfeststellung und die Modellierung. Nachweis: Sicherheitskonzept-Dokument inkl. IT-Grundschutz-Check.",
			"Sicherheitsmanagement", "manual", 3),
		c("BSI-ISMS.1.A9", "Management-Review",
			"Die Leitung führt mindestens jährlich ein dokumentiertes Management-Review der Informationssicherheit durch (Risiken, Vorfälle, Kennzahlen, Verbesserungen). Nachweis: Protokoll, Maßnahmenliste.",
			"Sicherheitsmanagement", "manual", 2),

		// ── ORP: Organisation und Personal ──
		c("BSI-ORP.1.A1", "Festlegung von Sicherheitsrollen",
			"Definiere die Sicherheitsrollen (ISB, IT-Verantwortliche, Auditoren) schriftlich mit klaren Aufgaben und Befugnissen. Nachweis: Stellenbeschreibungen, Organigramm, Rollenmatrix.",
			"Organisation", "manual", 2),
		c("BSI-ORP.2.A1", "Auswahl von Mitarbeitenden",
			"Berücksichtige bei der Einstellung von Personen mit Zugang zu schützenswerten Informationen Eignungsprüfungen (Referenzen, ggf. Führungszeugnis). Dokumentiere den Prozess. Nachweis: Personalprozess, Stichproben.",
			"Personal", "manual", 1),
		c("BSI-ORP.2.A2", "Verpflichtung von Mitarbeitenden",
			"Verpflichte Mitarbeitende vor Aufnahme der Tätigkeit schriftlich auf Vertraulichkeit, Datenschutz und Compliance-Vorgaben. Nachweis: unterzeichnete Vertraulichkeitsverpflichtungen.",
			"Personal", "manual", 2),
		c("BSI-ORP.3.A1", "Sensibilisierung und Schulung",
			"Schule mindestens jährlich alle Mitarbeitenden in Informationssicherheit (Phishing, Passwörter, Social Engineering). Nachweis: Teilnehmerlisten, Trainingsunterlagen.",
			"Personal", "manual", 2),
		c("BSI-ORP.4.A1", "Identitäts- und Berechtigungsmanagement",
			"Lege Identitätslebenszyklen (Beantragen, Genehmigen, Wieder-Entziehen) für alle Systeme schriftlich fest. Berechtigungen folgen dem Need-to-Know-Prinzip. Nachweis: IAM-Richtlinie, Berechtigungsmatrix, Rezertifizierung.",
			"Organisation", "manual", 3),

		// ── CON: Konzeption und Vorgehensweise ──
		c("BSI-CON.1.A1", "Kryptokonzept",
			"Erstelle ein Kryptokonzept, das eingesetzte Verfahren, Schlüssellängen und Algorithmen den Vorgaben von BSI-TR-02102 entsprechend beschreibt. Nachweis: Kryptokonzept-Dokument, Cipher-Suite-Konfiguration.",
			"Konzeption", "manual", 2),
		c("BSI-CON.2.A1", "Datenschutz",
			"Stelle DSGVO-konforme Verarbeitung sicher (Art. 5, 25, 32 DSGVO). Pflege Verzeichnis der Verarbeitungstätigkeiten, prüfe TOMs jährlich. Nachweis: VVT, TOM-Dokumentation, DSFA (sofern erforderlich).",
			"Konzeption", "manual", 3),
		c("BSI-CON.3.A1", "Datensicherungskonzept",
			"Definiere ein Datensicherungskonzept (Häufigkeit, Aufbewahrung, Offsite-Speicherung, Tests). Beachte die 3-2-1-Regel (3 Kopien, 2 Medien, 1 offsite). Teste Restore mindestens halbjährlich. Nachweis: Konzeptdokument, Backup-Logs, Restore-Test-Protokolle.",
			"Konzeption", "automated", 3),
		c("BSI-CON.8.A1", "Sichere Software-Entwicklung",
			"Etabliere einen sicheren SDLC mit Threat-Modeling, Code-Reviews, SAST/DAST und Dependency-Scanning. Nachweis: Entwicklungsrichtlinie, CI-Pipeline mit Security-Scanning, Pen-Test-Berichte.",
			"Konzeption", "automated", 2),

		// ── OPS: Betrieb ──
		c("BSI-OPS.1.1.2.A1", "Ordnungsgemäße IT-Administration",
			"Trenne administrative Tätigkeiten von der täglichen Arbeit (separate Admin-Accounts, MFA-Pflicht für privilegierte Zugriffe). Dokumentiere alle Admin-Tätigkeiten. Nachweis: Admin-Account-Liste, MFA-Konfiguration, Audit-Log.",
			"Betrieb", "automated", 3),
		c("BSI-OPS.1.1.3.A1", "Patch- und Änderungsmanagement",
			"Implementiere ein dokumentiertes Patch- und Change-Management. Kritische Sicherheitsupdates innerhalb von 7 Tagen, sonstige innerhalb von 30 Tagen. Nachweis: Patch-Richtlinie, Change-Tickets, Vulnerability-Scans.",
			"Betrieb", "automated", 3),
		c("BSI-OPS.1.1.4.A1", "Schutz vor Schadprogrammen",
			"Setze auf allen Endpunkten und Servern zentral verwaltete Antimalware-Lösungen ein. Signaturen täglich aktualisieren. Nachweis: AV-Konfiguration, Verteil-Logs, Inzident-Statistik.",
			"Betrieb", "automated", 2),
		c("BSI-OPS.1.1.5.A1", "Protokollierung",
			"Protokolliere sicherheitsrelevante Ereignisse zentral (Logins, Admin-Aktionen, Konfigurationsänderungen). Speichere Logs manipulationssicher (WORM oder Hash-Chain). Aufbewahrung mind. 1 Jahr. Nachweis: SIEM-Konfiguration, Logging-Konzept, Log-Stichproben.",
			"Betrieb", "automated", 3),

		// ── DER: Detektion und Reaktion ──
		c("BSI-DER.1.A1", "Detektion sicherheitsrelevanter Ereignisse",
			"Implementiere ein Verfahren zur Erkennung von Sicherheitsvorfällen (SIEM-Korrelationen, IDS/IPS, Anomalie-Erkennung). Dokumentiere die Schwellwerte und Alarme. Nachweis: SIEM-Use-Cases, IDS-Rulebase.",
			"Detektion", "automated", 3),
		c("BSI-DER.2.1.A1", "Behandlung von Sicherheitsvorfällen",
			"Etabliere einen Incident-Response-Prozess mit Eskalationsmatrix, Kommunikationsplan und Meldungspflichten (BSI / Datenschutzaufsicht binnen 72h). Trainiere ihn jährlich. Nachweis: IR-Playbook, Tabletop-Exercise-Protokolle, Meldungs-Templates.",
			"Reaktion", "manual", 3),
		c("BSI-DER.2.2.A1", "Vorsorge für IT-Forensik",
			"Bereite die Beweissicherung im Vorfeld vor (Forensik-Toolkits, Snapshot-Verfahren, Chain-of-Custody). Schule mindestens eine Person in der Beweissicherung. Nachweis: Forensik-Prozessdokumentation, Toolkit-Inventar.",
			"Reaktion", "manual", 2),

		// ── APP: Anwendungen ──
		c("BSI-APP.1.1.A1", "Sichere Office-Konfiguration",
			"Härtet Office-Anwendungen (Makros standardmäßig deaktiviert, Block bei externen Quellen, geschützte Ansicht). Nachweis: GPO-Konfiguration, MDM-Profil, Audit-Stichprobe.",
			"Anwendungen", "automated", 2),
		c("BSI-APP.4.4.A1", "Härtung von Active Directory / Identity Provider",
			"Härtet das zentrale IdP (Active Directory, Casdoor, Keycloak): privilegierte Konten mit MFA, Tier-Modell, regelmäßige Anti-Kerberos-Roasting-Audits. Nachweis: Tier-Konzept, Audit-Berichte, BloodHound-Reports.",
			"Anwendungen", "automated", 3),
		c("BSI-APP.5.3.A1", "Schutz von E-Mail-Kommunikation",
			"Implementiere SPF, DKIM, DMARC für eigene Domains. Schule Mitarbeitende in Erkennung von Phishing (verknüpft mit Vakt-Aware). Nachweis: DNS-Konfiguration, Phishing-Übungs-Reports.",
			"Anwendungen", "automated", 2),

		// ── SYS: IT-Systeme ──
		c("BSI-SYS.1.1.A1", "Allgemeine Server-Härtung",
			"Härte Server gem. CIS-Benchmarks oder BSI-Empfehlungen (deaktiviere Standarddienste, prüfe Patches, beschränke Login-Wege). Nachweis: Hardening-Guide, Compliance-Scans (CIS/Lynis).",
			"IT-Systeme", "automated", 2),
		c("BSI-SYS.1.2.A1", "Windows Server",
			"Setze unterstützte Windows-Server-Versionen ein, deaktiviere SMBv1, aktiviere Credential Guard und LSA Protection. Nachweis: Versions-Inventar, GPO-Konfiguration, BSI-Härtungsbericht.",
			"IT-Systeme", "automated", 2),
		c("BSI-SYS.1.3.A1", "Linux-Server",
			"Härte Linux-Server (SELinux/AppArmor enforced, SSH key-only, fail2ban, Login-Banner). Patche kritische Kernel-CVEs innerhalb 7 Tagen. Nachweis: Konfigurationsdateien, Patch-Reports.",
			"IT-Systeme", "automated", 2),
		c("BSI-SYS.2.2.3.A1", "Windows-Clients",
			"Aktiviere Windows-Defender, BitLocker für mobile Geräte, Application-Allowlisting. Setze Standard-User ohne lokale Admin-Rechte ein. Nachweis: GPO-Konfiguration, Compliance-Scan.",
			"IT-Systeme", "automated", 2),

		// ── IND: Industrielle IT (OT) ──
		c("BSI-IND.1.A1", "Schutz von Prozessleittechnik",
			"Trenne OT-Netze strikt von IT-Netzen (DMZ, unidirektionale Gateways wo möglich). Inventarisiere alle OT-Komponenten. Nachweis: Netz-Diagramm, OT-Asset-Inventar, Penetrationstests.",
			"Industrielle IT", "manual", 2),

		// ── NET: Netze und Kommunikation ──
		c("BSI-NET.1.1.A1", "Netzarchitektur und -design",
			"Erstelle eine dokumentierte Netzarchitektur mit Zonenmodell (DMZ, Internes Netz, Management-Netz). Beachte das Prinzip der minimalen Sichtbarkeit. Nachweis: Architektur-Diagramm, Firewall-Regelwerk.",
			"Netze", "manual", 3),
		c("BSI-NET.1.2.A1", "Netzmanagement",
			"Verwalte alle Netzkomponenten zentral aus einem dedizierten Management-Netz. SNMPv3, kein Telnet, keine Defaultpasswörter. Nachweis: Konfigurations-Backup, Management-Netz-Diagramm.",
			"Netze", "automated", 2),
		c("BSI-NET.3.1.A1", "Router und Switches",
			"Härte Router und Switches: Default-Passwörter ändern, SSH/HTTPS-only, ACLs gegen Spoofing, BPDU-Guard, Port-Security. Nachweis: Konfigurations-Auditberichte.",
			"Netze", "automated", 2),
		c("BSI-NET.3.2.A1", "Firewall",
			"Betreibe eine zentrale Firewall mit Default-Deny. Regelwerk halbjährlich überprüfen, Änderungen via Change-Management. Nachweis: Regelwerk-Export, Change-Log, Review-Protokoll.",
			"Netze", "automated", 3),

		// ── INF: Infrastruktur ──
		c("BSI-INF.1.A1", "Allgemeines Gebäude",
			"Sichere Zutritt zu Gebäuden mit Sensitivität durch Zutrittskontrollen (Schlüsselsystem, Karten, Logs). Pflege ein Zutrittsregister. Nachweis: Zutrittskonzept, Logs, Schlüsselverwaltung.",
			"Infrastruktur", "manual", 2),
		c("BSI-INF.2.A1", "Rechenzentrum",
			"Stelle physische Sicherheit des Rechenzentrums sicher (Zutrittskontrolle, Klima, Brandschutz, USV, Einbruchmeldeanlage). Bei Cloud: Provider-Zertifikate prüfen (ISO 27001, BSI C5). Nachweis: RZ-Sicherheitskonzept oder Cloud-Provider-Audit-Report.",
			"Infrastruktur", "manual", 3),
		c("BSI-INF.10.A1", "Besprechungs-, Veranstaltungs- und Schulungsräume",
			"Definiere Schutzanforderungen für Besprechungsräume (kein WLAN-Stick, verdeckte Whiteboard-Inhalte, Sperrbildschirm bei Verlassen). Nachweis: Hausordnung, Stichproben-Audit.",
			"Infrastruktur", "manual", 1),

		// ── Erweiterung ISMS: weitere A-Anforderungen ──
		c("BSI-ISMS.1.A2", "Beauftragung eines IS-Beauftragten",
			"Beauftrage einen Informationssicherheitsbeauftragten (ISB) schriftlich, stelle ausreichende Ressourcen bereit und positioniere ihn hierarchisch angemessen. Nachweis: Beauftragungsschreiben, Organigramm, Stellenbeschreibung.",
			"Sicherheitsmanagement", "manual", 3),
		c("BSI-ISMS.1.A4", "Ressourcen für Informationssicherheit",
			"Stelle sicher, dass ausreichende personelle, finanzielle und infrastrukturelle Ressourcen für den Betrieb des ISMS bereitgestellt werden. Nachweis: Ressourcenplanung, Budgetnachweis, ISB-Stunden.",
			"Sicherheitsmanagement", "manual", 2),
		c("BSI-ISMS.1.A5", "IS-Leitlinie und Ziele kommunizieren",
			"Kommuniziere die IS-Leitlinie und die konkreten Sicherheitsziele aktiv an alle Mitarbeitenden (z.B. Intranet, Onboarding, Schulungen). Nachweis: Kommunikationsnachweise, Onboarding-Unterlagen.",
			"Sicherheitsmanagement", "manual", 2),
		c("BSI-ISMS.1.A10", "Überprüfung und Steuerung des ISMS",
			"Führe regelmäßige interne IS-Audits durch und steuere das ISMS durch KPIs und Kennzahlen. Berichte an die Leitung. Nachweis: Auditplan, Auditberichte, KPI-Dashboard.",
			"Sicherheitsmanagement", "manual", 2),

		// ── ORP: Erweiterung ──
		c("BSI-ORP.2.A3", "Einweisung neuer Mitarbeitender",
			"Weise neue Mitarbeitende beim Onboarding auf IS-Regeln, Ansprechpartner und ihre Pflichten hin. Nachweis: Onboarding-Checkliste mit IS-Einweisung, Teilnahmebestätigung.",
			"Personal", "manual", 2),
		c("BSI-ORP.2.A4", "Regelung für Telearbeit und mobiles Arbeiten",
			"Lege verbindliche Regeln für Heimarbeit und mobiles Arbeiten fest (VPN-Pflicht, Sichtschutz, Speicherung auf freigegebenen Systemen). Nachweis: Telearbeitsrichtlinie, MDM-Konfiguration.",
			"Personal", "manual", 2),

		// ── CON: Erweiterung ──
		c("BSI-CON.7.A1", "Regelung zur Nutzung mobiler Datenträger",
			"Definiere und kommuniziere Regeln für den Einsatz mobiler Datenträger (USB-Sticks, externe Festplatten). Lege fest, welche Geräte erlaubt sind und wie sie verschlüsselt werden müssen. Nachweis: Richtlinie, technische Durchsetzung (Device-Control), Stichproben.",
			"Konzeption", "automated", 2),
		c("BSI-CON.9.A1", "Regelungen zu Weitergabe und Veröffentlichung von Informationen",
			"Lege fest, welche Informationen an wen und über welche Kanäle weitergegeben werden dürfen. Stelle sicher, dass Klassifizierungsregeln eingehalten werden. Nachweis: Informationsklassifizierungsrichtlinie, Schulungsnachweise.",
			"Konzeption", "manual", 2),
		c("BSI-CON.10.A1", "Entwicklung und Beschaffung — Sicherheitsanforderungen",
			"Definiere IS-Anforderungen vor der Entwicklung oder Beschaffung neuer IT-Systeme und Software (Security-by-Design, Threat Modeling). Nachweis: Anforderungsdokument, Beschaffungs-Checkliste mit IS-Kriterien.",
			"Konzeption", "manual", 2),

		// ── OPS: Erweiterung ──
		c("BSI-OPS.1.2.5.A1", "Planung und Konzeption der Fernwartung",
			"Lege fest, welche Systeme wie und von wem remote gewartet werden dürfen. Erzwinge MFA, verschlüsselte Verbindungen und vollständiges Logging aller Fernwartungssitzungen. Nachweis: Fernwartungskonzept, VPN/PAM-Konfiguration, Sitzungsprotokolle.",
			"Betrieb", "automated", 3),
		c("BSI-OPS.2.2.A1", "Sorgfaltspflichten bei Cloud-Nutzung",
			"Prüfe Cloud-Anbieter auf IS-Konformität (ISO 27001, BSI C5, SOC 2) vor Vertragsabschluss. Regele Datenstandort, AVV, Auditrechte und Portabilität. Nachweis: Anbieter-Prüfbericht, AVV, Cloud-Nutzungskonzept.",
			"Betrieb", "third_party", 3),

		// ── APP: Erweiterung ──
		c("BSI-APP.2.1.A1", "Planung des Verzeichnisdienstes",
			"Plane und betreibe Verzeichnisdienste (Active Directory, LDAP, Entra ID) sicher: Tier-Modell, minimale Admin-Rechte, regelmäßige Bereinigung verwaister Konten. Nachweis: AD-Konzept, Tier-Modell-Dokumentation, Auditberichte.",
			"Anwendungen", "automated", 3),
		c("BSI-APP.3.1.A1", "Authentifizierung bei Web-Anwendungen",
			"Implementiere sichere Authentifizierung in Web-Anwendungen: starke Passwörter, MFA, Session-Management (kurze Timeouts, Secure/HttpOnly-Cookies, CSRF-Schutz). Nachweis: OWASP-Konformitätsprüfung, Pentest-Bericht.",
			"Anwendungen", "automated", 3),
		c("BSI-APP.3.2.A1", "Absicherung von Webservern",
			"Härte Webserver: TLS 1.2+, keine veralteten Cipher-Suites, Security-Header (HSTS, CSP, X-Frame-Options), automatische Zertifikatserneuerung. Nachweis: SSL-Labs-Scan, Security-Header-Test, Konfigurationsdokumentation.",
			"Anwendungen", "automated", 2),

		// ── SYS: Erweiterung ──
		c("BSI-SYS.1.5.A1", "Planung von Virtualisierung",
			"Plane und betreibe Virtualisierungsinfrastruktur sicher: Hypervisor-Härtung, VM-Isolation, separate Verwaltungsnetze. Patchzyklus für Hypervisoren einhalten. Nachweis: Virtualisierungskonzept, Hypervisor-Konfiguration, Patch-Protokoll.",
			"IT-Systeme", "automated", 2),
		c("BSI-SYS.3.2.A1", "Anforderungen an Mobile Device Management",
			"Verwalte alle mobilen Endgeräte (Smartphones, Tablets, Laptops) über eine zentrale MDM-Lösung: Geräteverschlüsselung, Remote-Wipe, PIN-Pflicht, App-Whitelist. Nachweis: MDM-Konfiguration, Geräte-Compliance-Bericht.",
			"IT-Systeme", "automated", 3),

		// ── DER: Erweiterung ──
		c("BSI-DER.4.A1", "Notfallmanagementplan",
			"Erstelle einen Notfallmanagementplan (BCM-Plan) mit klar definierten Rollen, Reaktionszeiten (RTO, RPO) und Eskalationswegen. Teste ihn mindestens jährlich. Nachweis: BCM-Plan-Dokument, Übungsprotokolle.",
			"Reaktion", "manual", 3),

		// ── NET: Erweiterung ──
		c("BSI-NET.4.1.A1", "Absicherung von Sprach- und Datenkommunikation",
			"Schütze Telefon- und Videokonferenzsysteme vor Abhören und Manipulation: verschlüsselte Übertragung (SRTP, ZRTP), Authentifizierung, Patch-Management für Kommunikationssysteme. Nachweis: Systemkonfiguration, Protokoll-Analyse.",
			"Netze", "automated", 2),
	}
}

// craControls returns controls for the EU Cyber Resilience Act (CRA, 2024).
// Applies to manufacturers of products with digital elements sold in the EU.
func craControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 13 — Pflichten der Hersteller
		c("CRA-1.1", "Sicherheit durch Design (Security by Design)",
			"Integriere Sicherheitsanforderungen bereits in der Entwurfsphase des Produkts. Nachweis: Threat-Modeling-Dokument, Sicherheitsarchitektur, Design-Review-Protokoll.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.2", "Risikobewertung für Produkte mit digitalen Elementen",
			"Führe eine Cybersecurity-Risikobewertung für dein Produkt durch und dokumentiere identifizierte Risiken und Gegenmaßnahmen. Nachweis: Risikoanalyse-Bericht.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.3", "Schwachstellenbehandlungsrichtlinie (PSIRT)",
			"Richte einen Product Security Incident Response Team (PSIRT)-Prozess ein. Definiere Reaktionszeiten und Kommunikationswege für gemeldete Schwachstellen. Nachweis: PSIRT-Richtlinie, Responsible-Disclosure-Policy.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.4", "Software-Stückliste (SBOM)",
			"Erstelle und pflege eine vollständige Software Bill of Materials (SBOM) für jede Produktversion im SPDX- oder CycloneDX-Format. Nachweis: SBOM-Datei, Automatisierungsnachweis im CI/CD.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.5", "Sichere Standardkonfiguration (Secure by Default)",
			"Stelle sicher, dass das Produkt in der Standardkonfiguration sicher ist (keine Standard-Passwörter, minimale offene Ports, Least Privilege). Nachweis: Konfigurationsdokumentation, Hardening-Guide.",
			"Produktsicherheit", "manual", 2),
		c("CRA-1.6", "Sicherheitsupdates und Patch-Management",
			"Stelle sicher, dass Sicherheitsupdates für mindestens 5 Jahre nach Markteinführung bereitgestellt werden. Nachweis: Update-Richtlinie, Patch-Veröffentlichungsprozess.",
			"Produktsicherheit", "manual", 3),
		c("CRA-1.7", "Schutz vor bekannten Schwachstellen",
			"Scanne alle Abhängigkeiten regelmäßig auf bekannte CVEs und behebe kritische Schwachstellen innerhalb definierter Fristen. Nachweis: Dependency-Scan-Berichte, CVE-Tracking.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.8", "Sichere Authentifizierung und Zugangskontrolle",
			"Implementiere sichere Authentifizierungsmechanismen im Produkt (keine Hardcoded-Credentials, MFA-Unterstützung, Least Privilege). Nachweis: Authentifizierungskonzept, Code-Review.",
			"Produktsicherheit", "automated", 3),
		c("CRA-1.9", "Datenschutz und Datenverschlüsselung",
			"Schütze Nutzerdaten durch Verschlüsselung (at rest und in transit). Minimiere Datenerhebung (Privacy by Design). Nachweis: Datenschutzarchitektur, Verschlüsselungsdokumentation.",
			"Produktsicherheit", "automated", 2),
		c("CRA-1.10", "Protokollierung und Überwachbarkeit",
			"Implementiere sicherheitsrelevante Protokollierung im Produkt, die Angriffe und Fehlverhalten erkennbar macht. Nachweis: Logging-Konzept, Protokollbeispiele.",
			"Produktsicherheit", "automated", 2),
		// Art. 14 — Meldepflichten
		c("CRA-2.1", "Meldung aktiv ausgenutzter Schwachstellen (ENISA)",
			"Melde aktiv ausgenutzter Schwachstellen innerhalb von 24 Stunden an ENISA bzw. die nationale CSIRT. Nachweis: Meldeprozessdokumentation, Meldungsarchiv.",
			"Meldepflichten", "manual", 3),
		c("CRA-2.2", "Schwachstellen-Offenlegungspolitik (VDP)",
			"Veröffentliche eine Vulnerability Disclosure Policy (VDP) und stelle Sicherheitsforschern einen sicheren Meldeweg bereit. Nachweis: Öffentliche VDP-Seite, security.txt.",
			"Meldepflichten", "manual", 2),
		c("CRA-2.3", "Koordinierte Schwachstellenoffenlegung (CVD)",
			"Koordiniere die Offenlegung von Schwachstellen mit Meldenden nach anerkanntem CVD-Prozess (z.B. ISO 29147). Nachweis: CVD-Prozessdokumentation.",
			"Meldepflichten", "manual", 2),
		// Anhang I — Sicherheitsanforderungen
		c("CRA-3.1", "Sichere Entwicklungsprozesse (SDLC)",
			"Integriere Security-Testing (SAST, DAST, Dependency Scanning, Fuzz Testing) in den Entwicklungslebenszyklus. Nachweis: CI/CD-Pipeline-Konfiguration, Test-Berichte.",
			"Entwicklungsprozess", "automated", 3),
		c("CRA-3.2", "Penetrationstests",
			"Führe regelmäßige Penetrationstests für das Produkt durch (mind. jährlich oder nach wesentlichen Änderungen). Nachweis: Pentest-Berichte, Maßnahmentracking.",
			"Entwicklungsprozess", "manual", 2),
		c("CRA-3.3", "Konfigurationsmanagement und Härtung",
			"Dokumentiere sichere Konfigurationsempfehlungen für Betreiber. Vermeide unsichere Protokolle und Dienste im Auslieferungszustand. Nachweis: Hardening-Guide, Konfigurationsbaseline.",
			"Entwicklungsprozess", "manual", 2),
		// Anhang I Part II — Anforderungen an die Schwachstellenbehandlung
		c("CRA-3.4", "Exploit-Mitigation und Speicherschutz (Annex I Part II)",
			"Stelle sicher, dass das Produkt Speicherschutz-Mechanismen nutzt (ASLR, Stack Canaries, DEP/NX) und keine bekannten Exploit-Muster enthält. Nachweis: Compiler-Flags-Konfiguration, Binary-Hardening-Scan (checksec), Release-Notes.",
			"Entwicklungsprozess", "automated", 2),
		c("CRA-3.5", "Sichere Update-Mechanismen (Annex I Part II)",
			"Implementiere sichere, authentifizierte Update-Mechanismen mit kryptographischer Signierung aller Updates. Vermeide Downgrade-Angriffe durch Versionsvalidierung. Nachweis: Update-Signierungskonzept, Signaturprüfung-Tests.",
			"Entwicklungsprozess", "automated", 3),
		c("CRA-3.6", "Schnittstellen-Minimierung und Angriffsflächen-Reduktion (Annex I Part II)",
			"Begrenze die Angriffsfläche des Produkts: deaktiviere nicht benötigte Dienste, Ports und Protokolle im Auslieferungszustand. Dokumentiere alle aktiven Schnittstellen. Nachweis: Portscanning-Bericht, Interface-Dokumentation, Release-Konfiguration.",
			"Entwicklungsprozess", "automated", 2),
		// Anhang II — Nutzerinformationen und Bedienungsanleitung
		c("CRA-4.1", "Nutzerinformationen und Bedienungsanleitung (Annex II Nr. 1)",
			"Stelle Nutzern verständliche Informationen bereit: Zweck des Produkts, sichere Inbetriebnahme, Konfigurationsempfehlungen, bekannte Sicherheitseinschränkungen, EOL-Datum. Nachweis: Nutzerhandbuch, Online-Dokumentation, Release-Begleitdokumentation.",
			"Nutzerinformationen", "manual", 2),
		c("CRA-4.2", "Sicherheitskonfigurationsanleitung (Annex II Nr. 2)",
			"Erstelle eine klare Anleitung zur sicheren Konfiguration des Produkts für Betreiber: Passwort-Anforderungen, Netzwerk-Konfiguration, Logging-Empfehlungen, Härtungsmaßnahmen. Nachweis: Sicherheits-Setup-Guide, Schritt-für-Schritt-Härtungsanleitung.",
			"Nutzerinformationen", "manual", 2),
		c("CRA-4.3", "Kontaktinformationen für Schwachstellenmeldungen (Annex II Nr. 5)",
			"Veröffentliche Kontaktinformationen für Sicherheitsforscher und Nutzer, um Schwachstellen zu melden (security.txt, dedizierte E-Mail-Adresse, Bug-Bounty-Programm). Nachweis: Öffentliche VDP-Seite, security.txt-Datei, Reaktionszeitnachweis.",
			"Nutzerinformationen", "manual", 3),
		c("CRA-4.4", "EOL-Kommunikation und Support-Zeitraum (Annex II Nr. 6)",
			"Informiere Nutzer rechtzeitig (mind. 12 Monate vorher) über das geplante End-of-Life des Produkts oder einzelner Versionen. Benenne den verfügbaren Support-Zeitraum für Sicherheitsupdates (mind. 5 Jahre). Nachweis: EOL-Richtlinie, Kommunikationsnachweise.",
			"Nutzerinformationen", "manual", 2),
	}
}

// doraControls returns controls for DORA — Digital Operational Resilience Act (EU 2022/2554).
// Applies to financial entities (banks, insurers, investment firms, fintechs) and their ICT providers.
func doraControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 5-16 — ICT-Risikomanagement
		c("DORA-1.1", "ICT-Risikomanagement-Framework",
			"Implementiere ein umfassendes ICT-Risikomanagement-Framework gem. Art. 5 DORA. Identifiziere, klassifiziere und manage alle ICT-Risiken. Nachweis: ICT-Risikoregister, Framework-Dokumentation.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.2", "ICT-Strategie und Governance",
			"Stelle sicher, dass die Geschäftsleitung die digitale Resilienzstrategie trägt und überwacht. Nachweis: Management-Beschlüsse, Strategie-Dokument, Governance-Struktur.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.3", "Asset-Inventar (ICT-Assets)",
			"Führe ein vollständiges, aktuelles Inventar aller ICT-Assets und deren Abhängigkeiten. Nachweis: Asset-Register mit Klassifizierung und letztem Aktualisierungsdatum.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.4", "Schutzmaßnahmen und Prävention",
			"Implementiere technische und organisatorische Maßnahmen zum Schutz kritischer ICT-Systeme. Nachweis: Maßnahmenkatalog, Technische Konfigurationen.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.5", "Erkennung von ICT-Anomalien und -Vorfällen",
			"Implementiere Systeme zur frühzeitigen Erkennung von Anomalien, Cyberangriffen und ICT-Vorfällen (SIEM, IDS/IPS). Nachweis: SIEM-Konfiguration, Alarmierungsprotokoll.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.6", "ICT-Business-Continuity-Management",
			"Erstelle und teste BCM-Pläne für alle kritischen ICT-Systeme. Definiere RTO und RPO. Nachweis: BCM-Plan, Testergebnisse, RTO/RPO-Dokumentation.",
			"ICT-Risikomanagement", "manual", 3),
		c("DORA-1.7", "Backup und Wiederherstellung",
			"Implementiere regelmäßige Backups mit verifizierten Wiederherstellungstests. Nachweis: Backup-Konfiguration, Restore-Test-Protokolle.",
			"ICT-Risikomanagement", "automated", 3),
		c("DORA-1.8", "Patch- und Schwachstellenmanagement",
			"Scanne regelmäßig auf Schwachstellen und stelle zeitnahes Patching sicher. Nachweis: Scan-Berichte, Patch-Protokoll mit Fristen.",
			"ICT-Risikomanagement", "automated", 2),
		// Art. 17-23 — ICT-bezogenes Vorfallmanagement
		c("DORA-2.1", "Klassifizierung von ICT-Vorfällen",
			"Klassifiziere ICT-Vorfälle nach den DORA-Kriterien (Art. 18) hinsichtlich Schwere und Auswirkung. Nachweis: Klassifizierungsschema, Anwendungsbeispiele.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.2", "Meldung schwerwiegender ICT-Vorfälle (BaFin/EBA)",
			"Melde schwerwiegende ICT-Vorfälle fristgerecht an die zuständige Aufsichtsbehörde (BaFin, EBA, ECB). Nachweis: Meldetemplate, Meldungsarchiv.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.3", "Incident-Response-Prozess",
			"Definiere klare Prozesse für Erkennung, Eindämmung, Beseitigung und Nachbereitung von ICT-Vorfällen. Nachweis: IR-Richtlinie, Playbooks, Eskalationsmatrix.",
			"Vorfallmanagement", "manual", 3),
		c("DORA-2.4", "Post-Incident-Review",
			"Führe nach jedem schwerwiegenden Vorfall eine strukturierte Nachbereitung durch (Root Cause Analysis, Lessons Learned). Nachweis: Review-Berichte.",
			"Vorfallmanagement", "manual", 2),
		// Art. 24-27 — Digital Operational Resilience Testing
		c("DORA-3.1", "Jährliche ICT-Resilienz-Tests",
			"Führe jährliche Resilienz-Tests aller kritischen ICT-Systeme durch (Vulnerability Assessments, Penetrationstests). Nachweis: Testpläne, Berichte.",
			"Resilienztests", "manual", 3),
		c("DORA-3.2", "Threat-Led Penetration Testing (TLPT)",
			"Führe für systemrelevante Institute alle 3 Jahre DORA-konforme TLPT durch. Nachweis: TLPT-Bericht (von akkreditiertem Anbieter).",
			"Resilienztests", "manual", 2),
		c("DORA-3.3", "Szenarienbasierte Resilienztests",
			"Simuliere realistische Angriffsszenarien (Red-Team-Übungen, Tabletop-Exercises) und dokumentiere Ergebnisse. Nachweis: Übungsberichte.",
			"Resilienztests", "manual", 2),
		// Art. 28-44 — IKT-Drittparteienrisiken
		c("DORA-4.1", "IKT-Drittparteienrisiko-Management",
			"Implementiere ein formales Management-Framework für IKT-Drittparteienrisiken. Nachweis: Drittparteienregister, Risikobewertungsmatrix.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.2", "Vertragsanforderungen für IKT-Drittanbieter",
			"Stelle sicher, dass alle IKT-Dienstleisterverträge die DORA-Mindestanforderungen (Art. 30) erfüllen. Nachweis: Vertragsvorlagen, Prüfnachweis.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.3", "Ausstiegsstrategie für kritische IKT-Drittanbieter",
			"Entwickle Ausstiegsstrategien für kritische IKT-Abhängigkeiten. Nachweis: Exit-Plan-Dokument.",
			"Drittparteienrisiken", "manual", 2),
		// Art. 28 — Register + Konzentrationsrisiko
		c("DORA-4.4", "Register der IKT-Drittanbieter (Art. 28 Abs. 3)",
			"Führe ein vollständiges, aktuelles Register aller IKT-Drittanbieter mit Angabe zu kritischen und nicht-kritischen Anbietern, vertraglichem Scope und Abhängigkeitsprofil. Nachweis: Drittanbieter-Register gemäß Art. 28(3) DORA, letzte Aktualisierung.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.5", "Vertragsklauseln für kritische IKT-Drittanbieter (Art. 30)",
			"Stelle sicher, dass Verträge mit kritischen IKT-Drittanbietern alle Mindestklauseln nach Art. 30 DORA enthalten: Verfügbarkeits-SLAs, Datenlokation, Auditrechte, Incident-Meldung, Kooperationspflicht mit Aufsicht, Unterlizenzierungsbeschränkungen. Nachweis: Vertragsvorlagen, Konformitätscheckliste.",
			"Drittparteienrisiken", "manual", 3),
		c("DORA-4.6", "Konzentrationsrisiko und Diversifizierungsstrategie (Art. 29)",
			"Analysiere und manage IKT-Konzentrationsrisiken (zu hohe Abhängigkeit von einzelnen Anbietern). Definiere Diversifizierungsziele und Ausweichlieferanten für kritische IKT-Funktionen. Nachweis: Konzentrationsrisikoanalyse, Strategie-Dokument.",
			"Drittparteienrisiken", "manual", 2),
		// Art. 45-49 — Informationsaustausch (Säule 5)
		c("DORA-5.1", "Rechtsrahmen für Informationsaustausch (Art. 45)",
			"Richte einen rechtlich abgesicherten Rahmen für den freiwilligen Informationsaustausch zu Cyberbedrohungen ein. Stelle sicher, dass datenschutzrechtliche Anforderungen (DSGVO) eingehalten werden. Nachweis: Rechtliche Prüfung, Datenschutz-Policy für ISAC-Teilnahme.",
			"Informationsaustausch", "manual", 2),
		c("DORA-5.2", "Teilnahme an Bedrohungsinformationsaustausch (Art. 46)",
			"Nimm an einem oder mehreren ISAC (Information Sharing and Analysis Centers) oder ähnlichen Strukturen des Finanzsektors teil. Teile Erkenntnisse zu Cyberbedrohungen und Schwachstellen mit Peers. Nachweis: ISAC-Mitgliedschaft, eingereichte Meldungen, empfangene Threat-Intel-Berichte.",
			"Informationsaustausch", "manual", 2),
		c("DORA-5.3", "Weitergabe von Bedrohungsinformationen an Behörden (Art. 47)",
			"Melde relevante Bedrohungsinformationen an zuständige Aufsichtsbehörden (BaFin, EBA) und CERT/CSIRT, sofern dies für die Sicherheit des Finanzsystems relevant ist. Nachweis: Meldeprozess-Dokumentation, Meldungsarchiv.",
			"Informationsaustausch", "manual", 1),
	}
}

// doraISO27001Mapping maps each DORA control code to the corresponding ISO 27001:2022 Annex A clauses.
var doraISO27001Mapping = map[string]string{
	"DORA-1.1": "A.5.30, A.8.6, A.6.4",
	"DORA-1.2": "A.5.1, A.5.2, A.6.1",
	"DORA-1.3": "A.8.1, A.8.2",
	"DORA-1.4": "A.8.7, A.8.8, A.8.20",
	"DORA-1.5": "A.8.15, A.8.16",
	"DORA-1.6": "A.8.13, A.8.14",
	"DORA-1.7": "A.8.13",
	"DORA-1.8": "A.8.8, A.8.19",
	"DORA-2.1": "A.5.24, A.5.25",
	"DORA-2.2": "A.5.24, A.5.26",
	"DORA-2.3": "A.5.26, A.5.27",
	"DORA-2.4": "A.5.27",
	"DORA-3.1": "A.5.36, A.8.8",
	"DORA-3.2": "A.8.8",
	"DORA-3.3": "A.5.36",
	"DORA-4.1": "A.5.19, A.5.20",
	"DORA-4.2": "A.5.20, A.5.21",
	"DORA-4.3": "A.5.19",
	"DORA-4.4": "A.5.19, A.5.20",
	"DORA-4.5": "A.5.20, A.5.21",
	"DORA-4.6": "A.5.19",
	"DORA-5.1": "A.5.5, A.6.1",
	"DORA-5.2": "A.5.5",
	"DORA-5.3": "A.5.5, A.5.24",
}

// euAiActControls returns controls for the EU AI Act (Verordnung (EU) 2024/1689).
// Focuses on high-risk AI systems (Annex III) and general-purpose AI models.
func euAiActControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 9 — Risikomanagementsystem
		c("AIACT-1.1", "KI-Risikomanagementsystem",
			"Implementiere ein dokumentiertes Risikomanagementsystem für Hochrisiko-KI-Systeme gem. Art. 9 EU AI Act. Identifiziere bekannte und vorhersehbare Risiken. Nachweis: Risikoregister, Framework-Dokumentation.",
			"Risikomanagement", "manual", 3),
		c("AIACT-1.2", "KI-Risikobewertung und Risikominderung",
			"Bewerte Risiken für Gesundheit, Sicherheit und Grundrechte. Implementiere Maßnahmen zur Risikominderung. Nachweis: Risikobewertungsbericht, Maßnahmenkatalog.",
			"Risikomanagement", "manual", 3),
		c("AIACT-1.3", "Klassifizierung des KI-Systems",
			"Klassifiziere alle eingesetzten KI-Systeme nach EU AI Act (verboten / Hochrisiko / begrenztes Risiko / minimales Risiko). Nachweis: Klassifizierungsmatrix mit Begründungen.",
			"Risikomanagement", "manual", 3),
		// Art. 10 — Daten und Datenverwaltung
		c("AIACT-2.1", "Qualität der Trainingsdaten",
			"Stelle sicher, dass Trainingsdaten relevant, repräsentativ und frei von systematischen Fehlern sind. Nachweis: Daten-Governance-Dokumentation, Datenqualitätsbericht.",
			"Datenverwaltung", "manual", 3),
		c("AIACT-2.2", "Datenverwaltung und Datensätze",
			"Dokumentiere Herkunft, Umfang und Verarbeitungsmethoden aller für KI verwendeten Datensätze. Nachweis: Daten-Lineage-Dokumentation, Datensatz-Inventar.",
			"Datenverwaltung", "manual", 2),
		// Art. 11 — Technische Dokumentation
		c("AIACT-3.1", "Technische Dokumentation (Annex IV)",
			"Erstelle die technische Dokumentation gem. Anhang IV EU AI Act für alle Hochrisiko-KI-Systeme. Nachweis: Technisches Dossier.",
			"Dokumentation", "manual", 3),
		c("AIACT-3.2", "Konformitätserklärung und CE-Kennzeichnung",
			"Stelle eine EU-Konformitätserklärung aus und bringe für einschlägige Hochrisiko-KI-Systeme die CE-Kennzeichnung an. Nachweis: Konformitätserklärung, Kennzeichnungsnachweis.",
			"Dokumentation", "manual", 2),
		// Art. 12 — Aufzeichnungspflichten (Logging)
		c("AIACT-4.1", "Automatisches Logging des KI-Systems",
			"Implementiere automatisches Logging für alle Hochrisiko-KI-Systeme, das Ereignisse aufzeichnet, die für Überwachung und nachträgliche Untersuchung relevant sind. Nachweis: Logging-Konzept, Protokollbeispiele.",
			"Transparenz & Logging", "automated", 3),
		// Art. 13 — Transparenz und Nutzerinformation
		c("AIACT-5.1", "Transparenz gegenüber Nutzern",
			"Informiere Nutzer klar darüber, dass sie mit einem KI-System interagieren, und stelle verständliche Informationen über Fähigkeiten und Grenzen bereit. Nachweis: Nutzerdokumentation, Informationsmaterial.",
			"Transparenz & Logging", "manual", 2),
		c("AIACT-5.2", "Kennzeichnung KI-generierter Inhalte",
			"Kennzeichne KI-generierte Inhalte (insb. Deepfakes, synthetische Medien) als solche. Nachweis: Technische Implementierung, Richtlinie.",
			"Transparenz & Logging", "manual", 2),
		// Art. 14 — Menschliche Aufsicht
		c("AIACT-6.1", "Menschliche Aufsicht (Human Oversight)",
			"Stelle sicher, dass Hochrisiko-KI-Systeme wirksam von Menschen überwacht werden können und Stopp-Mechanismen vorhanden sind. Nachweis: Aufsichtskonzept, Nachweis der Implementierung.",
			"Menschliche Aufsicht", "manual", 3),
		c("AIACT-6.2", "Schulung der Aufsichtspersonen",
			"Schule alle Personen, die KI-Systeme überwachen, zu deren Fähigkeiten, Grenzen und möglichen Risiken. Nachweis: Schulungsnachweise, Schulungsmaterial.",
			"Menschliche Aufsicht", "manual", 2),
		// Art. 15 — Genauigkeit, Robustheit und Cybersicherheit
		c("AIACT-7.1", "Genauigkeit und Leistungsmetriken",
			"Definiere und überwache Genauigkeitsmetriken für Hochrisiko-KI-Systeme. Nachweis: Leistungsberichte, Benchmark-Ergebnisse.",
			"Sicherheit & Robustheit", "automated", 2),
		c("AIACT-7.2", "Robustheit gegen adversarielle Angriffe",
			"Teste das KI-System auf Robustheit gegen adversarielle Eingaben und Data-Poisoning. Nachweis: Robustheitstests, Red-Team-Berichte.",
			"Sicherheit & Robustheit", "manual", 2),
		c("AIACT-7.3", "Cybersicherheit des KI-Systems",
			"Stelle sicher, dass das KI-System gegen Cyberangriffe geschützt ist (sichere API, Authentifizierung, Eingabevalidierung). Nachweis: Security-Review, Pentest-Bericht.",
			"Sicherheit & Robustheit", "manual", 3),
		// Art. 26 — Pflichten der Nutzer von Hochrisiko-KI-Systemen
		c("AIACT-8.1", "Konformitätsbewertung vor Inbetriebnahme",
			"Führe vor dem Einsatz von Hochrisiko-KI-Systemen eine Konformitätsbewertung durch. Nachweis: Konformitätsbewertungsbericht.",
			"Compliance", "manual", 3),
		c("AIACT-8.2", "Einschränkung auf vorgesehene Verwendung",
			"Stelle sicher, dass KI-Systeme ausschließlich für ihren vorgesehenen Verwendungszweck eingesetzt werden. Nachweis: Nutzungsrichtlinie, Schulungsnachweise.",
			"Compliance", "manual", 2),
		// Art. 17 — Qualitätsmanagementsystem
		c("AIACT-9.1", "Qualitätsmanagementsystem für Hochrisiko-KI (Art. 17)",
			"Implementiere ein dokumentiertes Qualitätsmanagementsystem für die Entwicklung, das Deployment und die Überwachung von Hochrisiko-KI-Systemen. Das QMS muss Strategie, Prozesse, Ressourcen, Verantwortlichkeiten und kontinuierliche Verbesserung umfassen. Nachweis: QMS-Dokumentation, Versionierung, Management-Review.",
			"Qualitätsmanagement", "manual", 3),
		c("AIACT-9.2", "QMS-Verfahrensanweisungen und Aufzeichnungen (Art. 17)",
			"Erstelle verbindliche Verfahrensanweisungen für alle sicherheitsrelevanten Phasen des KI-Lebenszyklus (Datenaufbereitung, Training, Validierung, Deployment, Monitoring) und bewahre alle Aufzeichnungen für die Aufsicht auf. Nachweis: Verfahrensanweisungen, Aufzeichnungsregister, Aufbewahrungsrichtlinie.",
			"Qualitätsmanagement", "manual", 3),
		c("AIACT-9.3", "Post-Market-Monitoring-Plan (Art. 17 Abs. 1 lit. n)",
			"Erstelle und betreibe einen Post-Market-Monitoring-Plan für alle am Markt platzierten Hochrisiko-KI-Systeme: kontinuierliches Performance-Monitoring, Drifterkennung, Kundenfeedback-Auswertung. Nachweis: Monitoring-Plan, Dashboard, Alerting-Konfiguration.",
			"Qualitätsmanagement", "automated", 2),
		// Art. 18 — Dokumentationspflichten
		c("AIACT-10.1", "Aufbewahrung der technischen Dokumentation (Art. 18)",
			"Bewahre die technische Dokumentation (Annex IV) und alle QMS-Aufzeichnungen für Hochrisiko-KI-Systeme mindestens 10 Jahre nach Markteinführung oder letztem Einsatz auf. Stelle Zugänglichkeit für Aufsichtsbehörden sicher. Nachweis: Archivierungsrichtlinie, Zugriffsprotokoll.",
			"Dokumentation", "manual", 2),
		// Titel VIII — Allzweck-KI (GPAI, Art. 51–56)
		c("AIACT-11.1", "Klassifizierung von GPAI-Modellen (Art. 51)",
			"Klassifiziere alle eingesetzten General-Purpose-AI-Modelle (GPAI) nach EU AI Act: Modelle mit systemischem Risiko (Rechenaufwand > 10^25 FLOP) vs. sonstige. Nachweis: GPAI-Modell-Register, Klassifizierungsmatrix, Anbieter-Dokumentation.",
			"GPAI-Compliance", "manual", 2),
		c("AIACT-11.2", "Urheberrecht und Trainingsdaten für GPAI (Art. 53)",
			"Stelle sicher, dass GPAI-Modelle, die du einsetzt oder entwickelst, eine Richtlinie zur Einhaltung des Urheberrechts bei Trainingsdaten haben. Nachweis: Trainingsdaten-Policy, Herkunftsdokumentation, Opt-out-Dokumentation.",
			"GPAI-Compliance", "manual", 2),
		c("AIACT-11.3", "Systemische Risiken bei GPAI-Modellen (Art. 55)",
			"Bewerte und manage systemische Risiken bei GPAI-Modellen mit systemischem Risiko: adversarielle Tests, Incident-Reporting an EU-Behörden, Cybersicherheitsmaßnahmen. Nachweis: Risikobewertungsbericht, Test-Protokolle, Incident-Meldungsverfahren.",
			"GPAI-Compliance", "manual", 2),
	}
}

// iso42001Controls returns controls for ISO/IEC 42001:2023 — AI Management System Standard.
func iso42001Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Kap. 4 — Kontext der Organisation
		c("42001-4.1", "Verständnis der Organisation und ihres Kontexts",
			"Bestimme interne und externe Faktoren, die für den KI-Managementsystem-Zweck relevant sind. Nachweis: Kontextanalyse-Dokument.",
			"Organisationskontext", "manual", 2),
		c("42001-4.2", "Interessierte Parteien und deren Anforderungen",
			"Identifiziere alle relevanten Stakeholder (Nutzer, Regulatoren, Betroffene) und deren Anforderungen an das KI-MS. Nachweis: Stakeholder-Register.",
			"Organisationskontext", "manual", 2),
		c("42001-4.3", "KI-Politik und Anwendungsbereich",
			"Definiere den Anwendungsbereich des KI-Managementsystems und erstelle eine KI-Politik. Nachweis: KI-Politik-Dokument, Anwendungsbereichsdefinition.",
			"Organisationskontext", "manual", 2),
		// Kap. 5 — Führung
		c("42001-5.1", "Führung und Commitment für KI-Governance",
			"Stelle sicher, dass die Unternehmensführung Verantwortung für das KI-Managementsystem übernimmt. Nachweis: Management-Beschlüsse, Governance-Dokument.",
			"Führung", "manual", 3),
		c("42001-5.2", "KI-Rollen und Verantwortlichkeiten",
			"Weise klare Rollen und Verantwortlichkeiten für KI-Entwicklung, -Betrieb und -Governance zu. Nachweis: Organigramm, Stellenbeschreibungen, Beauftragungsschreiben.",
			"Führung", "manual", 2),
		// Kap. 6 — Planung
		c("42001-6.1", "KI-Risikobeurteilung",
			"Identifiziere und bewerte Risiken aus dem Einsatz von KI-Systemen, einschließlich ethischer und gesellschaftlicher Risiken. Nachweis: KI-Risikoregister.",
			"Planung", "manual", 3),
		c("42001-6.2", "KI-Ziele und Maßnahmen",
			"Definiere messbare KI-Ziele und leite konkrete Maßnahmen zur Zielerreichung ab. Nachweis: Zieldokument, Maßnahmenplan.",
			"Planung", "manual", 2),
		// Kap. 7 — Unterstützung
		c("42001-7.1", "Kompetenz und Schulung für KI",
			"Stelle sicher, dass alle Personen, die KI-Systeme entwickeln, betreiben oder überwachen, ausreichend kompetent sind. Nachweis: Schulungspläne, Kompetenzmatrix.",
			"Unterstützung", "manual", 2),
		c("42001-7.2", "Bewusstsein für KI-Risiken",
			"Sensibilisiere alle Mitarbeitenden für KI-spezifische Risiken und ethische Aspekte. Nachweis: Awareness-Materialien, Schulungsnachweise.",
			"Unterstützung", "manual", 2),
		c("42001-7.3", "Dokumentenlenkung für KI-Artefakte",
			"Führe und kontrolliere alle KI-relevanten Dokumente (Modelle, Daten, Entscheidungen) gemäß Dokumentenlenkungsverfahren. Nachweis: Dokumentenregister, Versionskontrolle.",
			"Unterstützung", "manual", 1),
		// Kap. 8 — Betrieb
		c("42001-8.1", "KI-Lebenszyklusmanagement",
			"Manage alle KI-Systeme über ihren vollständigen Lebenszyklus (Konzeption, Entwicklung, Deployment, Betrieb, Abkündigung). Nachweis: Lebenszyklusplan, Abkündigungsrichtlinie.",
			"Betrieb", "manual", 3),
		c("42001-8.2", "KI-Impact-Assessment",
			"Führe vor der Inbetriebnahme neuer KI-Systeme ein Impact Assessment durch (ethisch, gesellschaftlich, sicherheitsbezogen). Nachweis: Assessment-Bericht.",
			"Betrieb", "manual", 3),
		c("42001-8.3", "Responsible AI — Fairness und Nicht-Diskriminierung",
			"Teste KI-Systeme auf systematische Diskriminierung (Bias) und dokumentiere Maßnahmen zur Fairness-Sicherstellung. Nachweis: Bias-Testing-Berichte, Fairness-Metriken.",
			"Betrieb", "manual", 3),
		c("42001-8.4", "Erklärbarkeit von KI-Entscheidungen",
			"Stelle sicher, dass KI-Entscheidungen in für Nutzer verständlicher Form erklärt werden können (Explainability/XAI). Nachweis: Erklärbarkeits-Konzept, Beispiele.",
			"Betrieb", "manual", 2),
		c("42001-8.5", "Überwachung und Monitoring von KI-Systemen",
			"Implementiere laufendes Monitoring der KI-System-Performance und -Drift. Nachweis: Monitoring-Dashboard, Alerting-Konfiguration.",
			"Betrieb", "automated", 2),
		// Kap. 9 — Leistungsbewertung
		c("42001-9.1", "Interne Audits des KI-Managementsystems",
			"Führe regelmäßige interne Audits des KI-MS durch. Nachweis: Auditplan, Auditberichte, Maßnahmentracking.",
			"Leistungsbewertung", "manual", 2),
		c("42001-9.2", "Management-Review für KI-Governance",
			"Halte mindestens jährlich ein Management-Review des KI-MS ab. Nachweis: Review-Protokoll, Entscheidungsdokumentation.",
			"Leistungsbewertung", "manual", 2),
		// Kap. 10 — Verbesserung
		c("42001-10.1", "Kontinuierliche Verbesserung des KI-MS",
			"Etabliere einen systematischen KVP für das KI-Managementsystem. Nachweis: Verbesserungsmaßnahmen-Tracking.",
			"Verbesserung", "manual", 1),

		// ── Annex A — Spezifische Referenzmaßnahmen für KI-Managementsysteme ──
		// A.2 — Ziele für verantwortungsvolle KI-Entwicklung und -Nutzung
		c("42001-A2.1", "Richtlinie für verantwortungsvolle KI",
			"Erstelle eine KI-Ethikrichtlinie, die Grundsätze für den verantwortungsvollen Einsatz von KI definiert: Fairness, Transparenz, Nicht-Diskriminierung, Erklärbarkeit, Datenschutz. Nachweis: genehmigtes Richtliniendokument, Kommunikationsnachweis.",
			"Annex A — Verantwortungsvolle KI", "manual", 3),
		c("42001-A2.2", "Messbare KI-Nachhaltigkeitsziele",
			"Definiere messbare Ziele für den nachhaltigen und ethischen Einsatz von KI (z.B. Reduktion von Bias-Metriken um X%, Erreichung von XAI-Schwellwerten). Nachweis: Zieldokument mit messbaren KPIs, Fortschrittsbericht.",
			"Annex A — Verantwortungsvolle KI", "manual", 2),
		// A.3 — Risiko- und Impact-Assessment für KI
		c("42001-A3.1", "KI-Risikobewertungsverfahren",
			"Etabliere ein formales Verfahren zur Identifikation und Bewertung von KI-spezifischen Risiken: technische Risiken (Modell-Drift, Datenvergiftung), ethische Risiken (Bias, Diskriminierung), gesellschaftliche Risiken. Nachweis: Risikobewertungsverfahren, ausgefüllte Risikoregister.",
			"Annex A — Risiko & Impact", "manual", 3),
		c("42001-A3.2", "KI-Impact-Assessment-Verfahren",
			"Führe vor der Inbetriebnahme neuer KI-Systeme ein strukturiertes Impact Assessment durch, das gesellschaftliche, rechtliche und ethische Auswirkungen bewertet. Nachweis: Impact-Assessment-Vorlage, Assessment-Berichte.",
			"Annex A — Risiko & Impact", "manual", 3),
		c("42001-A3.3", "Betroffenenfolgenabschätzung",
			"Bewerte systematisch die Auswirkungen von KI-Systementscheidungen auf betroffene Personen und Gruppen — insbesondere vulnerable Gruppen. Nachweis: Folgenabschätzungs-Bericht, Maßnahmenkatalog.",
			"Annex A — Risiko & Impact", "manual", 3),
		// A.4 — KI-System-Lebenszyklus
		c("42001-A4.1", "Design und Spezifikation von KI-Systemen",
			"Dokumentiere Anforderungen, Designentscheidungen und Architektur von KI-Systemen vor der Entwicklung. Berücksichtige IS- und Datenschutzanforderungen (Privacy by Design, Security by Design). Nachweis: System-Designdokument, Anforderungsspezifikation.",
			"Annex A — Lebenszyklus", "manual", 3),
		c("42001-A4.2", "Daten-Pipeline und Trainingsprozess",
			"Dokumentiere die gesamte Daten-Pipeline: Datenquellen, Vorverarbeitung, Augmentation, Labeling-Prozesse. Stelle Reproduzierbarkeit und Nachvollziehbarkeit des Trainingsprozesses sicher. Nachweis: Pipeline-Dokumentation, Experiment-Tracking (MLflow o.ä.).",
			"Annex A — Lebenszyklus", "manual", 2),
		c("42001-A4.3", "Verifikation und Validierung von KI-Modellen",
			"Validiere KI-Modelle systematisch vor dem Deployment: Leistungsmetriken, Robustheitstests, Bias-Evaluation, Out-of-Distribution-Tests. Definiere Abnahmekriterien. Nachweis: Validierungsberichte, Testprotokolle, Go/No-Go-Entscheidungsdokumentation.",
			"Annex A — Lebenszyklus", "automated", 3),
		c("42001-A4.4", "Deployment und Inbetriebnahme von KI-Systemen",
			"Stelle sicher, dass KI-Systeme kontrolliert in Produktion übergehen: Genehmigungsprozess, Rollback-Fähigkeit, Canary-Deployment, Monitoring-Aktivierung. Nachweis: Deployment-Checkliste, Rollback-Plan, Monitoring-Dashboard.",
			"Annex A — Lebenszyklus", "automated", 3),
		c("42001-A4.5", "Außerbetriebnahme und Datenarchivierung",
			"Definiere einen geordneten Prozess für die Außerbetriebnahme von KI-Systemen: Datenlöschung oder -archivierung, Modell-Archivierung, Benachrichtigung der Stakeholder, Dokumentenaufbewahrung. Nachweis: Außerbetriebnahme-Richtlinie, Protokoll.",
			"Annex A — Lebenszyklus", "manual", 2),
		// A.5 — Daten für KI-Systeme
		c("42001-A5.1", "Anforderungen an Trainingsdaten",
			"Definiere Qualitätskriterien für Trainingsdaten: Repräsentativität, Vollständigkeit, Aktualität, Fehlerfreiheit. Dokumentiere Herkunft und Lizenzierung aller Datensätze. Nachweis: Daten-Governance-Richtlinie, Datensatz-Inventar mit Herkunftsangaben.",
			"Annex A — Daten", "manual", 3),
		c("42001-A5.2", "Datenqualität und Repräsentativität",
			"Stelle sicher, dass Trainingsdaten frei von systematischen Verzerrungen sind und alle relevanten Bevölkerungsgruppen und Szenarien angemessen repräsentieren. Nachweis: Datenqualitätsbericht, Bias-Analyse, Sampling-Protokoll.",
			"Annex A — Daten", "manual", 3),
		c("42001-A5.3", "Datenschutz in KI-Datenpipelines",
			"Implementiere Privacy-by-Design in allen KI-Datenpipelines: Anonymisierung/Pseudonymisierung wo möglich, Minimierung der Datenhaltung, DSGVO-konforme Verarbeitung. Nachweis: Datenschutz-Impact-Assessment (DPIA), Technische Maßnahmen, VVT-Eintrag.",
			"Annex A — Daten", "manual", 3),
		c("42001-A5.4", "Synthetische und augmentierte Daten",
			"Dokumentiere den Einsatz synthetischer oder augmentierter Daten im Training. Bewerte Auswirkungen auf Modell-Fairness und -Robustheit. Nachweis: Augmentierungs-Konzept, Fairness-Evaluation nach Augmentierung.",
			"Annex A — Daten", "manual", 2),
		// A.6 — Informationspflichten gegenüber Stakeholdern
		c("42001-A6.1", "Transparenz gegenüber interessierten Parteien",
			"Kommuniziere offen gegenüber Kunden, Nutzern, Regulatoren und der Öffentlichkeit über den Einsatz von KI-Systemen, deren Zweck, Fähigkeiten und Einschränkungen. Nachweis: Öffentliche KI-Transparenzerklärung, Kommunikationsplan.",
			"Annex A — Stakeholder", "manual", 3),
		c("42001-A6.2", "Kommunikation über KI-Systemfähigkeiten und -grenzen",
			"Stelle sicher, dass Nutzer und Entscheidungsträger vollständig über die Grenzen des KI-Systems informiert sind: Fehlermöglichkeiten, bekannte Schwächen, empfohlene menschliche Aufsicht. Nachweis: Nutzerdokumentation, Schulungsmaterial, Onboarding-Unterlagen.",
			"Annex A — Stakeholder", "manual", 2),
	}
}

// tisaxControls returns controls for TISAX® / VDA ISA 6.0.
// TISAX (Trusted Information Security Assessment Exchange) is mandatory for
// automotive suppliers handling sensitive OEM data (BMW, Mercedes, VW, Bosch, etc.).
func tisaxControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Kap. 1 — Informationssicherheitsrichtlinien
		c("TISAX-1.1.1", "IS-Politik und -Ziele definiert",
			"Definiere eine von der Unternehmensleitung unterzeichnete Informationssicherheitspolitik mit konkreten Schutzzielen und Geltungsbereich. Kommuniziere sie an alle Mitarbeitenden. Nachweis: genehmigtes IS-Politik-Dokument mit Datum und Unterschrift, Kommunikationsnachweis.",
			"Informationssicherheitsrichtlinien", "manual", 3),
		c("TISAX-1.1.2", "IS-Politik regelmäßig überprüft",
			"Überprüfe und aktualisiere die IS-Politik mindestens jährlich oder bei wesentlichen Änderungen der Organisation. Nachweis: Revisionshistorie mit Datum, Genehmigungsprotokoll der Unternehmensleitung.",
			"Informationssicherheitsrichtlinien", "manual", 2),
		c("TISAX-1.1.3", "Führung und Commitment der Unternehmensleitung",
			"Stelle sicher, dass die Unternehmensleitung aktiv die IS-Ziele unterstützt, ausreichende Ressourcen bereitstellt und die Wichtigkeit des ISMS kommuniziert. Nachweis: Management-Beschlüsse, Organigramm mit IS-Rolle.",
			"Informationssicherheitsrichtlinien", "manual", 3),

		// Kap. 2 — Organisation der Informationssicherheit
		c("TISAX-2.1.1", "Rollen und Verantwortlichkeiten IS",
			"Benenne einen Informationssicherheitsbeauftragten (ISB) und dokumentiere alle IS-Rollen mit Aufgaben und Befugnissen. Stelle Unabhängigkeit und ausreichende Ressourcen sicher. Nachweis: Beauftragungsschreiben, Stellenbeschreibungen, Organigramm.",
			"Organisation", "manual", 3),
		c("TISAX-2.1.2", "Kontakt zu Behörden und Fachgruppen",
			"Pflege aktuelle Kontaktinformationen zu relevanten Behörden (BSI, CERT-Bund) und Branchengruppen (VDA, ENX). Dokumentiere die Eskalationswege. Nachweis: Kontaktliste, Mitgliedschaftsnachweise.",
			"Organisation", "manual", 1),
		c("TISAX-2.1.3", "IS im Projektmanagement",
			"Integriere IS-Anforderungen in alle Projektphasen (Anforderungsanalyse, Design, Test, Abnahme). Stelle sicher, dass IS-Risiken in Projekten bewertet und behandelt werden. Nachweis: Projektcheckliste mit IS-Anforderungen, Review-Nachweise.",
			"Organisation", "manual", 2),
		c("TISAX-2.1.4", "Sicherheit beim mobilen Arbeiten",
			"Definiere Regeln und technische Maßnahmen für mobiles Arbeiten und Telearbeit (VPN, Geräteverschlüsselung, Clear-Screen). Nachweis: Mobile-Work-Richtlinie, MDM-Konfiguration, VPN-Setup.",
			"Organisation", "manual", 2),

		// Kap. 3 — Personalsicherheit
		c("TISAX-3.1.1", "Überprüfung vor der Anstellung",
			"Führe angemessene Hintergrundüberprüfungen (Lebenslauf, Zeugnisse, ggf. Führungszeugnis) vor der Einstellung durch, insbesondere für sicherheitskritische Positionen. Nachweis: Screening-Richtlinie, Dokumentation der Prüfung.",
			"Personalsicherheit", "manual", 2),
		c("TISAX-3.1.2", "IS-Bewusstsein und Schulung",
			"Schule alle Mitarbeitenden mit Zugang zu vertraulichen OEM-Informationen mindestens jährlich zu IS-Grundlagen, Umgang mit sensitiven Daten und Meldepflichten. Nachweis: Schulungsnachweise, Teilnehmerlisten, Schulungsinhalt.",
			"Personalsicherheit", "manual", 3),
		c("TISAX-3.1.3", "Disziplinarmaßnahmen bei IS-Verstößen",
			"Definiere und kommuniziere Konsequenzen bei Verstößen gegen die IS-Politik. Stelle sicher, dass Verstöße gemeldet und verfolgt werden. Nachweis: HR-Richtlinie mit Sanktionsregelung, Kommunikationsnachweis.",
			"Personalsicherheit", "manual", 2),
		c("TISAX-3.1.4", "Beendigung und Wechsel des Arbeitsverhältnisses",
			"Stelle beim Ausscheiden oder Rollenwechsel sicher, dass alle Zugänge gesperrt, Assets zurückgegeben und Vertraulichkeitspflichten kommuniziert werden. Nachweis: Offboarding-Checkliste mit Nachweisen.",
			"Personalsicherheit", "manual", 2),

		// Kap. 4 — Asset-Management
		c("TISAX-4.1.1", "Inventar der Informationsassets",
			"Führe ein vollständiges, aktuelles Inventar aller Informationsassets (Hardware, Software, Daten, Dienste) mit Eigentümer und Schutzbedarf. Nachweis: Asset-Register mit letztem Aktualisierungsdatum.",
			"Asset-Management", "automated", 3),
		c("TISAX-4.1.2", "Eigentümerschaft der Assets",
			"Weise jedem Asset einen verantwortlichen Eigentümer zu, der die Klassifizierung und Schutzmaßnahmen verantwortet. Nachweis: Asset-Register mit Eigentümer-Feld, Verantwortungsmatrix.",
			"Asset-Management", "manual", 2),
		c("TISAX-4.1.3", "Klassifizierung von Informationen",
			"Klassifiziere alle Informationen nach Schutzbedarf (mind. vertraulich/intern/öffentlich) basierend auf der Vereinbarung mit dem OEM. Beachte die VDA-Schutzklassen. Nachweis: Klassifizierungsrichtlinie, Beispiele klassifizierter Dokumente.",
			"Asset-Management", "manual", 3),
		c("TISAX-4.1.4", "Kennzeichnung von Informationen",
			"Kennzeichne alle sensitiven Dokumente und Datenträger gemäß ihrer Klassifizierung (Stempel, Metadaten, Dateinamen-Konvention). Nachweis: Kennzeichnungsrichtlinie, Beispieldokumente.",
			"Asset-Management", "manual", 2),
		c("TISAX-4.1.5", "Handhabung und Entsorgung von Assets",
			"Definiere Regeln für den sicheren Transport, die Handhabung und die datenschutzkonforme Entsorgung sensitiver Informationen und Datenträger. Nachweis: Handhabungsrichtlinie, Vernichtungsnachweise.",
			"Asset-Management", "manual", 2),

		// Kap. 5 — Zugangskontrolle
		c("TISAX-5.1.1", "Zugangskontrollrichtlinie",
			"Erstelle eine schriftliche Zugangskontrollrichtlinie nach dem Need-to-know- und Least-Privilege-Prinzip. Definiere Genehmigungsprozesse für Zugriffsrechte. Nachweis: genehmigtes Richtliniendokument.",
			"Zugangskontrolle", "manual", 3),
		c("TISAX-5.1.2", "Benutzerzugangsverwaltung",
			"Verwalte alle Benutzerkonten über einen definierten Prozess (Anlage, Änderung, Sperrung, Löschung). Überprüfe Zugriffsrechte mindestens halbjährlich. Nachweis: Provisionierungsprozess, Review-Protokolle.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.3", "Privilegierte Zugriffsrechte",
			"Verwalte Administrator- und Root-Rechte restriktiv. Nutze PAM-Lösung, Vier-Augen-Prinzip und vollständiges Logging für privilegierte Aktionen. Nachweis: PAM-Konfiguration, Admin-Protokolle.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.4", "Multi-Faktor-Authentifizierung",
			"Erzwinge MFA für den Zugriff auf Systeme mit vertraulichen OEM-Informationen und für alle Remote-Zugänge. Nachweis: MFA-Konfiguration, Ausnahmeliste mit Begründungen.",
			"Zugangskontrolle", "automated", 3),
		c("TISAX-5.1.5", "Zugang zu Netzwerken und Diensten",
			"Beschränke Netzwerkzugänge auf autorisierte Nutzer und Geräte (NAC, VPN, Zero Trust). Segmentiere Netzwerke nach Schutzbedarf. Nachweis: Netzwerkarchitektur, Zugangskontrollkonfiguration.",
			"Zugangskontrolle", "automated", 3),

		// Kap. 6 — Kryptographie
		c("TISAX-6.1.1", "Kryptographierichtlinie",
			"Definiere zulässige kryptographische Verfahren und Schlüssellängen (gemäß BSI TR-02102) für alle Anwendungsfälle. Schließe veraltete Algorithmen aus. Nachweis: Kryptographierichtlinie.",
			"Kryptographie", "manual", 2),
		c("TISAX-6.1.2", "Schlüsselverwaltung",
			"Dokumentiere den vollständigen Schlüssellebenszyklus (Generierung, Verteilung, Speicherung, Widerruf, Vernichtung). Nutze ein dediziertes Key-Management-System. Nachweis: Schlüsselverwaltungsverfahren, KMS-Konfiguration.",
			"Kryptographie", "manual", 2),
		c("TISAX-6.1.3", "Verschlüsselung sensitiver Daten",
			"Verschlüssele alle OEM-sensitiven Daten in Ruhe (AES-256) und bei der Übertragung (TLS 1.2+). Nachweis: Verschlüsselungskonfiguration, TLS-Scan-Bericht.",
			"Kryptographie", "automated", 3),

		// Kap. 7 — Physische Sicherheit
		c("TISAX-7.1.1", "Physischer Sicherheitsperimeter",
			"Definiere und sichere physische Sicherheitsbereiche (Serverräume, Büros, Entwicklungsbereiche) mit angemessenen Zugangskontrollen. Nachweis: Raumkonzept, Zutrittskontrollsystem-Dokumentation.",
			"Physische Sicherheit", "manual", 3),
		c("TISAX-7.1.2", "Zugangskontrollen für Sicherheitsbereiche",
			"Implementiere elektronische Zutrittskontrolle für Sicherheitsbereiche mit individueller Authentifizierung und Protokollierung. Beschränke den Zugang auf Befugte. Nachweis: Zutrittskontrollsystem, Zugangsprotokolle.",
			"Physische Sicherheit", "manual", 3),
		c("TISAX-7.1.3", "Sicherung von Geräten",
			"Schütze IT-Geräte physisch vor Diebstahl und unbefugtem Zugriff (Kabelsicherung, abschließbare Schränke, Bildschirmsperren). Nachweis: Sicherheitskonzept, Begehungsprotokoll.",
			"Physische Sicherheit", "manual", 2),
		c("TISAX-7.1.4", "Clear-Desk und Clear-Screen",
			"Setze Clear-Desk- und Clear-Screen-Richtlinien durch: automatische Bildschirmsperre, keine offengelegten sensitiven Dokumente. Nachweis: Richtlinie, Stichprobenprotokoll.",
			"Physische Sicherheit", "manual", 2),

		// Kap. 8 — Betriebssicherheit
		c("TISAX-8.1.1", "Dokumentierte Betriebsverfahren",
			"Erstelle und pflege aktuelle Betriebsdokumentation für alle kritischen IT-Systeme (Betriebshandbücher, Verfahrensanweisungen). Nachweis: Betriebsdokumentation mit Versionierung.",
			"Betriebssicherheit", "manual", 2),
		c("TISAX-8.1.2", "Änderungsmanagement",
			"Stelle sicher, dass alle Änderungen an IT-Systemen geplant, bewertet, genehmigt, getestet und dokumentiert werden. Nachweis: Change-Management-Prozess, Genehmigungsnachweise.",
			"Betriebssicherheit", "manual", 2),
		c("TISAX-8.1.3", "Schutz vor Schadsoftware",
			"Implementiere Endpoint-Protection-Software mit automatischen Updates auf allen Systemen mit OEM-Datenzugang. Ergänze durch EDR, E-Mail-Sicherheit und Web-Filtering. Nachweis: AV/EDR-Konfiguration, Update-Protokoll.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.4", "Datensicherung (Backup)",
			"Implementiere regelmäßige Backups nach 3-2-1-Prinzip mit Verschlüsselung. Teste die Wiederherstellung mindestens vierteljährlich. Nachweis: Backup-Konfiguration, Restore-Test-Protokolle.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.5", "Protokollierung und Überwachung",
			"Protokolliere sicherheitsrelevante Ereignisse auf allen kritischen Systemen und überwache sie zentral (SIEM). Bewahre Logs mindestens 90 Tage auf. Nachweis: Logging-Konfiguration, SIEM-Dashboard.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.6", "Schwachstellenmanagement",
			"Scanne Systeme regelmäßig auf bekannte Schwachstellen (mind. monatlich) und behebe kritische Schwachstellen innerhalb definierter Fristen. Nachweis: Scan-Berichte, Patch-Protokoll.",
			"Betriebssicherheit", "automated", 3),
		c("TISAX-8.1.7", "Trennung von Entwicklung, Test und Betrieb",
			"Trenne Entwicklungs-, Test- und Produktivumgebungen strikt. Verwende keine Produktionsdaten in Testumgebungen ohne Anonymisierung. Nachweis: Umgebungskonzept, Datenschutz-Maßnahmen.",
			"Betriebssicherheit", "manual", 2),

		// Kap. 9 — Kommunikationssicherheit
		c("TISAX-9.1.1", "Netzwerksicherheit und -segmentierung",
			"Segmentiere Netzwerke nach Schutzbedarf (DMZ, Produktions-/Entwicklungsnetz, OT-Trennung). Überwache Netzwerkverkehr auf Anomalien. Nachweis: Netzwerkplan, Firewall-Regeln, IDS-Konfiguration.",
			"Kommunikationssicherheit", "automated", 3),
		c("TISAX-9.1.2", "Sichere Datenübertragung",
			"Verschlüssele alle Übertragungen sensitiver OEM-Daten (TLS 1.2+, sichere Dateiübertragung). Schließe unsichere Protokolle (FTP, HTTP, Telnet) aus. Nachweis: Protokoll-Konfiguration, TLS-Scan.",
			"Kommunikationssicherheit", "automated", 3),
		c("TISAX-9.1.3", "Vertraulichkeitsvereinbarungen (NDAs)",
			"Stelle sicher, dass alle Personen mit Zugang zu OEM-sensitiven Informationen aktuelle NDAs unterzeichnet haben. Nachweis: NDA-Vorlagen, unterzeichnete Vereinbarungen.",
			"Kommunikationssicherheit", "manual", 3),

		// Kap. 10 — Systembeschaffung und -entwicklung
		c("TISAX-10.1.1", "Sicherheitsanforderungen für Systeme",
			"Definiere IS-Sicherheitsanforderungen vor der Beschaffung oder Entwicklung neuer Systeme, die sensitiven OEM-Daten verarbeiten. Nachweis: Anforderungsdokumentation, Beschaffungs-Checkliste.",
			"Systementwicklung", "manual", 2),
		c("TISAX-10.1.2", "Sichere Entwicklungsprozesse",
			"Integriere Sicherheit in den gesamten Entwicklungslebenszyklus (Secure SDLC): Threat Modeling, Security Code Reviews, SAST/DAST, Dependency Scanning. Nachweis: SDLC-Dokumentation, Tool-Konfiguration.",
			"Systementwicklung", "automated", 2),
		c("TISAX-10.1.3", "Sicherheitstests",
			"Führe vor jeder Produktivsetzung von Systemen mit OEM-Datenzugang Sicherheitstests durch (Penetrationstests, Schwachstellenscans). Nachweis: Testberichte, Testpläne.",
			"Systementwicklung", "manual", 2),

		// Kap. 11 — Lieferantenbeziehungen
		c("TISAX-11.1.1", "Lieferanten-Sicherheitsanforderungen",
			"Definiere IS-Mindestanforderungen für alle Lieferanten und Dienstleister mit Zugang zu sensitiven OEM-Informationen oder IS-relevanten Systemen. Nachweis: Lieferanten-Sicherheitsrichtlinie.",
			"Lieferantensicherheit", "manual", 3),
		c("TISAX-11.1.2", "Sicherheitsanforderungen in Lieferantenverträgen",
			"Verankere verbindliche IS-Anforderungen in allen relevanten Lieferantenverträgen (NDAs, AVV, Auditrechte, Vorfallmeldepflicht). Nachweis: Vertragsklauseln, Musterverträge.",
			"Lieferantensicherheit", "manual", 3),
		c("TISAX-11.1.3", "Überwachung der Lieferanten-IS-Leistung",
			"Überprüfe regelmäßig die IS-Leistung kritischer Lieferanten (Fragebögen, Audits, Zertifikate). Nachweis: Bewertungsberichte, Auditprotokolle, TISAX-Nachweise von Lieferanten.",
			"Lieferantensicherheit", "manual", 2),

		// Kap. 12 — Sicherheitsvorfälle
		c("TISAX-12.1.1", "Incident-Response-Prozess",
			"Definiere und dokumentiere einen Prozess zur Erkennung, Meldung, Bewertung, Reaktion und Nachbereitung von IS-Vorfällen. Stelle Erreichbarkeit des IR-Teams sicher. Nachweis: IR-Richtlinie, IR-Playbooks, Teambesetzungsplan.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.2", "Meldung von Vorfällen und Schwächen",
			"Etabliere einfache Meldekanäle für alle Mitarbeitenden zur Meldung von IS-Vorfällen und Schwachstellen. Garantiere Schutz vor Repressalien. Nachweis: Meldeprozess, Kontaktinformationen, Kommunikationsnachweis.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.3", "Meldepflicht gegenüber OEMs",
			"Stelle sicher, dass Vorfälle, die OEM-sensitive Daten betreffen, unverzüglich dem betroffenen OEM gemäß vertraglicher Vereinbarung gemeldet werden. Nachweis: Meldeprozess, OEM-Kontaktliste, Meldungsarchiv.",
			"Vorfallmanagement", "manual", 3),
		c("TISAX-12.1.4", "Post-Incident-Review und Lessons Learned",
			"Führe nach jedem wesentlichen Vorfall eine strukturierte Nachbereitung durch und implementiere Verbesserungsmaßnahmen. Nachweis: Post-Incident-Review-Berichte, Maßnahmentracking.",
			"Vorfallmanagement", "manual", 2),

		// Kap. 13 — Business Continuity
		c("TISAX-13.1.1", "Business-Continuity-Planung",
			"Erstelle BCM-Pläne für alle Geschäftsprozesse mit OEM-Datenzugang. Definiere RTO und RPO. Nachweis: BCM-Plan, BIA-Dokument, RTO/RPO-Tabelle.",
			"Business Continuity", "manual", 3),
		c("TISAX-13.1.2", "BCM-Tests und -Übungen",
			"Teste BCM-Pläne mindestens jährlich durch Übungen (Tabletop oder Live-Test) und dokumentiere Ergebnisse und Verbesserungen. Nachweis: Übungsprotokolle, Verbesserungsmaßnahmen.",
			"Business Continuity", "manual", 2),

		// Kap. 14 — Compliance
		c("TISAX-14.1.1", "Einhaltung gesetzlicher und vertraglicher Anforderungen",
			"Identifiziere alle anwendbaren gesetzlichen Anforderungen (DSGVO, Exportkontrolle) und vertraglichen Verpflichtungen gegenüber OEMs. Nachweis: Compliance-Register, rechtliche Prüfungsnachweise.",
			"Compliance", "manual", 3),
		c("TISAX-14.1.2", "Interne IS-Audits",
			"Führe mindestens jährlich interne IS-Audits durch und dokumentiere Befunde, Maßnahmen und Umsetzungsstatus. Nachweis: Auditplan, Auditberichte, Maßnahmentracking.",
			"Compliance", "manual", 3),
		c("TISAX-14.1.3", "TISAX-Assessment Vorbereitung",
			"Stelle sicher, dass alle TISAX-Anforderungen des gewählten Assessment-Levels (AL1/AL2/AL3) und der Schutzbedarfskategorie (Normal/Hoch/Sehr hoch) erfüllt sind. Nachweis: Gap-Analyse, Maßnahmenplan, Assessment-Bereitschaftsbericht.",
			"Compliance", "manual", 3),

		// Kap. 15 — Prototypenschutz (nur bei Prototypen-Schutzbedarf)
		c("TISAX-15.1.1", "Physische Absicherung von Prototypen",
			"Sichere Fahrzeugprototypen und Prototypenteile mit geeigneten physischen Maßnahmen (abgeschlossene Garagen, Zugangskontrolle, CCTV). Nachweis: Sicherheitskonzept Prototypenschutz, Begehungsprotokoll.",
			"Prototypenschutz", "manual", 3),
		c("TISAX-15.1.2", "Kennzeichnung von Prototypen",
			"Kennzeichne Prototypen und Prototypenteile gemäß OEM-Vorgaben (Tarnung, Abdeckungen, Kennzeichnungspflicht). Nachweis: Kennzeichnungsrichtlinie, Fotodokumentation.",
			"Prototypenschutz", "manual", 3),
		c("TISAX-15.1.3", "Transport von Prototypen",
			"Sichere den Transport von Prototypen durch geeignete Maßnahmen (abgedunkelter Transport, GPS-Tracking, Protokollierung). Nachweis: Transportrichtlinie, Transportprotokolle.",
			"Prototypenschutz", "manual", 2),
		c("TISAX-15.1.4", "Fotografierverbot und digitale Sicherheit",
			"Verbiete das unbefugte Fotografieren von Prototypen und treffe technische Maßnahmen gegen unbefugte Bildaufnahmen (Abschirmung, Kamerasperren in Sicherheitsbereichen). Nachweis: Richtlinie, technische Maßnahmen.",
			"Prototypenschutz", "manual", 3),

		// VDA ISA 6.0 — Connected Vehicles (Modul für vernetzte Fahrzeuge)
		c("TISAX-16.1.1", "Sicherheitsanforderungen für vernetzte Fahrzeuge",
			"Definiere und implementiere Sicherheitsanforderungen für Komponenten und Systeme vernetzter Fahrzeuge gemäß VDA ISA 6.0 und UN-R155/R156. Berücksichtige fahrzeugspezifische Angriffsvektoren (OBD, V2X, Telematik). Nachweis: Anforderungsdokument, Threat-Analysis (TARA nach ISO/SAE 21434).",
			"Connected Vehicles", "manual", 3),
		c("TISAX-16.1.2", "Schutz von Fahrzeugkommunikations-Schnittstellen",
			"Sichere alle Fahrzeugkommunikationsschnittstellen gegen unbefugten Zugriff: OBD-Härtung, V2X-Authentifizierung, Telematik-Verschlüsselung, Schutz von Diagnose-Zugängen. Nachweis: Schnittstellenkonzept, Penetrationstest-Berichte (Automotive-Pentest).",
			"Connected Vehicles", "manual", 3),
		c("TISAX-16.1.3", "Software-Update-Sicherheit (OTA)",
			"Stelle sicher, dass Over-the-Air (OTA) Software-Updates für Fahrzeugsysteme sicher sind: kryptographische Signierung, Downgrade-Schutz, Rollback-Fähigkeit, Ausfallsicherheit bei Update-Fehlschlag. Nachweis: OTA-Sicherheitskonzept, Signaturprüfung, Test-Berichte.",
			"Connected Vehicles", "automated", 3),

		// VDA ISA 6.0 — Reifegradmodell und Assessment-Scope-Dokumentation
		c("TISAX-0.1", "TISAX-Scope-Festlegung und Geltungsbereich",
			"Definiere und dokumentiere den TISAX-Assessment-Scope: betroffene Standorte, Systeme, Prozesse und Personengruppen mit Zugang zu OEM-sensitiven Informationen. Stimme den Scope mit dem TISAX-Auftraggeber (OEM) ab. Nachweis: Scope-Dokument, OEM-Bestätigung.",
			"Assessment-Scope", "manual", 3),
		c("TISAX-0.2", "VDA ISA Reifegrad-Selbstbewertung",
			"Führe eine Selbstbewertung anhand des VDA ISA 6.0 Reifegradmodells durch: Reifegrad 0 (nicht vorhanden) bis 5 (kontinuierlich optimiert). Dokumentiere Ist-Stand und Ziel-Reifegrad pro Kontrollbereich. Nachweis: Selbstbewertungsbericht, Gap-Analyse, Maßnahmenplan.",
			"Assessment-Scope", "manual", 3),
	}
}

func dsgvoTOMControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: "Technische und organisatorische Maßnahmen", EvidenceType: "manual", Weight: w}
	}
	return []Control{
		c("TOM-1", "Zutrittskontrolle", "Maßnahmen zur Verhinderung unbefugten Zutritts zu Datenverarbeitungsanlagen (Schlösser, Alarmanlagen, Zutrittskontrollen). Nachweis: Zutrittskonzept, Protokoll.", 3),
		c("TOM-2", "Zugangskontrolle", "Technische Maßnahmen zur Authentifizierung (Passwörter, MFA, Token). Nachweis: MFA-Konfiguration, Passwortrichtlinie.", 3),
		c("TOM-3", "Zugriffskontrolle", "Berechtigungskonzept nach Need-to-Know. Nur autorisierte Personen können auf personenbezogene Daten zugreifen. Nachweis: Berechtigungsmatrix.", 3),
		c("TOM-4", "Weitergabekontrolle", "Schutz bei Übertragung personenbezogener Daten (TLS, VPN, Verschlüsselung). Nachweis: Transportverschlüsselungs-Konfiguration.", 2),
		c("TOM-5", "Eingabekontrolle", "Protokollierung aller Eingaben, Änderungen und Löschungen personenbezogener Daten (Audit-Trail). Nachweis: Logging-Konzept, Log-Beispiele.", 2),
		c("TOM-6", "Auftragskontrolle", "Kontrolle von Auftragsverarbeitern: AVV abgeschlossen, Weisungsgebundenheit sichergestellt. Nachweis: AVV-Dokumente, Prüfnachweise.", 2),
		c("TOM-7", "Verfügbarkeitskontrolle", "Schutz vor Datenverlust durch Backup, Redundanz und Notfallkonzept. Nachweis: Backup-Protokolle, Recovery-Tests.", 3),
		c("TOM-8", "Trennungsgebot", "Personenbezogene Daten verschiedener Verantwortlicher/Zwecke werden getrennt verarbeitet. Nachweis: Architektur- oder Datenflussdokumentation.", 2),
		c("TOM-9", "Pseudonymisierung", "Personenbezogene Daten werden pseudonymisiert, soweit möglich. Nachweis: Pseudonymisierungskonzept, technische Umsetzung.", 2),
		c("TOM-10", "Verschlüsselung", "Verschlüsselung ruhender und übertragener personenbezogener Daten (AES-256 oder gleichwertig). Nachweis: Verschlüsselungskonzept, Konfiguration.", 3),
		c("TOM-11", "Integrität", "Sicherstellung, dass personenbezogene Daten nicht unbefugt verändert werden (Hashes, digitale Signaturen). Nachweis: Integritätskonzept.", 2),
		c("TOM-12", "Wiederherstellung", "Fähigkeit zur schnellen Wiederherstellung von Verfügbarkeit und Zugang nach Zwischenfällen. Nachweis: BCM-Plan, Wiederherstellungstests.", 3),
		c("TOM-13", "Überprüfungsverfahren", "Regelmäßige Überprüfung und Bewertung der Wirksamkeit der TOMs (mindestens jährlich). Nachweis: Prüfberichte, Revisionsprotokoll.", 2),
	}
}

// cisControls returns the CIS Controls v8 IG1 safeguards (basic hygiene for all orgs).
// Each control group (1–18) is represented by its key IG1 safeguards.
func cisControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain string, w int) Control {
		return Control{
			FrameworkID:  frameworkID,
			OrgID:        orgID,
			ControlID:    id,
			Title:        title,
			Description:  desc,
			Domain:       domain,
			EvidenceType: "manual",
			Weight:       w,
		}
	}
	return []Control{
		// CIS 1 — Inventarisierung und Kontrolle von Unternehmens-Assets
		c("CIS-1.1", "Inventarisierung von Unternehmens-Assets",
			"Erstellen und pflegen Sie eine präzise, detaillierte und aktuelle Bestandsaufnahme aller Unternehmens-Assets mit Zugang zu Infrastruktur, einschließlich End-User-Geräten, Netzwerkgeräten, IoT-Geräten und Servern. Nachweis: aktuelles Asset-Register mit Datum und Verantwortlichem.",
			"Asset-Inventarisierung", 3),
		c("CIS-1.2", "Adressierung nicht autorisierter Assets",
			"Stellen Sie sicher, dass ein Prozess existiert, um nicht autorisierte Assets zu identifizieren, zu isolieren oder zu entfernen. Nachweis: Eskalationsverfahren, CMDB-Prüfprotokoll.",
			"Asset-Inventarisierung", 2),
		c("CIS-1.3", "DHCP-Protokollierung für Asset-Erkennung nutzen",
			"Nutzen Sie DHCP-Protokolle zur Aktualisierung des Asset-Inventars. Nachweis: DHCP-Log-Konfiguration, automatischer Asset-Abgleich.",
			"Asset-Inventarisierung", 1),

		// CIS 2 — Inventarisierung und Kontrolle von Software-Assets
		c("CIS-2.1", "Inventarisierung von Software-Assets",
			"Erstellen und pflegen Sie eine aktuelle Liste genehmigter Software inkl. Versionsinformationen und Herstellerdaten. Nachweis: Software-Inventar, Lizenzübersicht.",
			"Software-Inventarisierung", 3),
		c("CIS-2.2", "Sicherstellen, dass autorisierte Software gepflegt wird",
			"Stellen Sie sicher, dass nur aktuell gewartete und unterstützte Software verwendet wird. Nachweis: EOL-Prüfbericht, Patch-Status-Übersicht.",
			"Software-Inventarisierung", 2),
		c("CIS-2.3", "Adressierung nicht autorisierter Software",
			"Stellen Sie sicher, dass nicht autorisierte Software zeitnah deinstalliert oder im Netzwerk isoliert wird. Nachweis: Richtlinie zur Softwarefreigabe, Prüfprotokoll.",
			"Software-Inventarisierung", 2),

		// CIS 3 — Datenschutz
		c("CIS-3.1", "Datenverwaltungsrichtlinie einrichten",
			"Etablieren Sie und pflegen Sie eine Daten-Management-Richtlinie, die Anforderungen an Klassifizierung, Aufbewahrung und Handhabung festlegt. Nachweis: genehmigtes Richtliniendokument.",
			"Datenschutz", 3),
		c("CIS-3.2", "Daten-Inventar einrichten und pflegen",
			"Inventarisieren Sie alle Datenbestände mit Klassifizierung, Eigentümer und Verarbeitungsort. Nachweis: Dateninventar, Datenflussdiagramm.",
			"Datenschutz", 2),
		c("CIS-3.3", "Daten auf Unternehmensgeräten schützen",
			"Schützen Sie alle Daten auf Unternehmensgeräten mit geeigneten Maßnahmen (Verschlüsselung, Zugriffskontrolle). Nachweis: Verschlüsselungsrichtlinie, MDM-Konfiguration.",
			"Datenschutz", 3),

		// CIS 4 — Sichere Konfiguration von Unternehmens-Assets und Software
		c("CIS-4.1", "Sichere Konfiguration einrichten und pflegen",
			"Erstellen Sie sichere Konfigurationsvorlagen für alle Unternehmens-Assets (CIS Benchmarks). Nachweis: Hardening-Baseline, Scan-Bericht.",
			"Sichere Konfiguration", 3),
		c("CIS-4.2", "Standardpasswörter ändern",
			"Ändern Sie alle Standard-Passwörter vor dem Einsatz. Nachweis: Inbetriebnahme-Checkliste, Passwortrichtlinie.",
			"Sichere Konfiguration", 3),
		c("CIS-4.3", "Automatische Sperrung von Sitzungen einrichten",
			"Konfigurieren Sie automatische Bildschirmsperren und Sitzungs-Timeouts auf allen Assets. Nachweis: MDM-Konfiguration, GPO-Einstellung.",
			"Sichere Konfiguration", 2),
		c("CIS-4.4", "Nicht benötigte Dienste, Protokolle und Ports deaktivieren",
			"Deaktivieren Sie nicht benötigte Netzwerkdienste, -protokolle und -ports auf allen Assets. Nachweis: Port-Scan-Bericht, Konfigurationsprüfung.",
			"Sichere Konfiguration", 2),

		// CIS 5 — Kontoverwaltung
		c("CIS-5.1", "Verfahren zur Kontoverwaltung einrichten",
			"Etablieren und pflegen Sie einen Prozess für die Erstellung, Verwendung, Verwaltung, Nachverfolgung und Löschung von Konten. Nachweis: IAM-Richtlinie, Onboarding/Offboarding-Verfahren.",
			"Kontoverwaltung", 3),
		c("CIS-5.2", "Nutzung privilegierter Konten kontrollieren",
			"Verwenden Sie privilegierte Konten nur für administrative Aufgaben. Nachweis: Inventar privilegierter Konten, PAM-Konfiguration.",
			"Kontoverwaltung", 3),
		c("CIS-5.3", "Nicht verwendete Konten deaktivieren",
			"Deaktivieren oder löschen Sie Konten nach einer definierten Inaktivitätsperiode. Nachweis: AD-Prüfbericht, Kontoreinigungs-Protokoll.",
			"Kontoverwaltung", 2),
		c("CIS-5.4", "Dienstkonten auf Dienste beschränken",
			"Beschränken Sie Dienstkonten auf den minimal notwendigen Zugang. Stellen Sie sicher, dass sie sich nicht interaktiv einloggen können. Nachweis: Dienstkonto-Inventar, Konfigurationsnachweis.",
			"Kontoverwaltung", 2),

		// CIS 6 — Zugriffskontrollmanagement
		c("CIS-6.1", "Zugriffsrechte nach Least Privilege einrichten",
			"Weisen Sie Benutzern und Systemen nur die minimal notwendigen Berechtigungen zu. Nachweis: Zugriffsrechte-Matrix, Berechtigungskonzept.",
			"Zugriffskontrolle", 3),
		c("CIS-6.2", "Zugriffsrechte regelmäßig überprüfen",
			"Führen Sie mindestens jährlich eine Überprüfung aller vergebenen Zugriffsrechte durch. Nachweis: Prüfprotokoll, Bereinigungsnachweise.",
			"Zugriffskontrolle", 2),
		c("CIS-6.3", "Multi-Faktor-Authentifizierung für alle Konten",
			"Aktivieren Sie MFA für alle Benutzerkonten — insbesondere für Remote-Zugang und privilegierte Konten. Nachweis: MFA-Konfiguration, Ausnahmeliste.",
			"Zugriffskontrolle", 3),

		// CIS 7 — Kontinuierliches Schwachstellenmanagement
		c("CIS-7.1", "Prozess zur Schwachstellenverwaltung einrichten",
			"Etablieren und pflegen Sie einen Schwachstellenmanagement-Prozess mit klar definierten Rollen, Prioritäten und Fristen. Nachweis: Prozessdokumentation, Verantwortlichkeitenmatrix.",
			"Schwachstellenmanagement", 3),
		c("CIS-7.2", "Automatisierte Patch-Verwaltung für Betriebssysteme",
			"Automatisieren Sie das Einspielen von Betriebssystem-Patches auf allen Assets. Nachweis: Patch-Management-Tool-Konfiguration, Compliance-Bericht.",
			"Schwachstellenmanagement", 3),
		c("CIS-7.3", "Automatisierte Patch-Verwaltung für Anwendungen",
			"Automatisieren Sie das Einspielen von Anwendungs-Patches auf allen Assets. Nachweis: Anwendungs-Patch-Bericht.",
			"Schwachstellenmanagement", 2),
		c("CIS-7.4", "Verwaltung von Sicherheitsupdates für Drittanbieter-Software",
			"Pflegen Sie Sicherheitsupdates für alle Drittanbieter-Software zeitnah ein (kritisch ≤ 72 h). Nachweis: SLA-Dokument, Umsetzungsnachweis.",
			"Schwachstellenmanagement", 2),

		// CIS 8 — Verwaltung von Audit-Logs
		c("CIS-8.1", "Audit-Log-Verwaltungsrichtlinie einrichten",
			"Erstellen und pflegen Sie eine Protokollverwaltungsrichtlinie mit Aufbewahrungsfristen, Schutz und Überprüfungsintervallen. Nachweis: Log-Richtlinie, SIEM-Architektur.",
			"Audit-Log-Verwaltung", 2),
		c("CIS-8.2", "Ereignisprotokolle sammeln",
			"Sammeln Sie Audit-Logs auf allen Unternehmens-Assets. Nachweis: Log-Konfiguration aller Systeme, SIEM-Einspeisung.",
			"Audit-Log-Verwaltung", 3),
		c("CIS-8.3", "Protokollierungsfähigkeit ausreichend dimensionieren",
			"Stellen Sie sicher, dass ausreichend Speicherkapazität für Protokolldaten bereitsteht. Nachweis: Storage-Monitoring, Kapazitätsplanung.",
			"Audit-Log-Verwaltung", 1),
		c("CIS-8.4", "Zentralisierte Log-Verwaltung aktivieren",
			"Zentralisieren Sie alle Logs in einem SIEM oder einer zentralen Log-Plattform. Nachweis: SIEM-Konfiguration, Log-Quellen-Liste.",
			"Audit-Log-Verwaltung", 2),

		// CIS 9 — E-Mail- und Webbrowser-Schutz
		c("CIS-9.1", "Nur vollständig unterstützte Browser und E-Mail-Clients nutzen",
			"Stellen Sie sicher, dass ausschließlich vollständig gepflegte und unterstützte Browser und E-Mail-Clients eingesetzt werden. Nachweis: Software-Inventar, EOL-Prüfung.",
			"E-Mail und Web-Schutz", 2),
		c("CIS-9.2", "DNS-Filterung nutzen",
			"Setzen Sie DNS-Filterung ein, um bösartige Domains zu blockieren. Nachweis: DNS-Filter-Konfiguration, Blacklist-Überblick.",
			"E-Mail und Web-Schutz", 2),
		c("CIS-9.3", "E-Mail-Authentifizierung einsetzen (DMARC, SPF, DKIM)",
			"Konfigurieren Sie SPF, DKIM und DMARC für alle eigenen Domains. Nachweis: DNS-Einträge, DMARC-Bericht.",
			"E-Mail und Web-Schutz", 3),

		// CIS 10 — Malware-Abwehr
		c("CIS-10.1", "Malware-Abwehr einsetzen",
			"Setzen Sie Anti-Malware-Software auf allen Unternehmens-Endgeräten ein. Stellen Sie automatische Signatur-Updates sicher. Nachweis: AV-Konfiguration, Scan-Berichte.",
			"Malware-Abwehr", 3),
		c("CIS-10.2", "Automatische Signaturaktualisierungen konfigurieren",
			"Konfigurieren Sie automatische Updates für alle Anti-Malware-Signaturen. Nachweis: Update-Richtlinie, Compliance-Scan.",
			"Malware-Abwehr", 2),
		c("CIS-10.3", "Autorun und Autoplay für Wechselmedien deaktivieren",
			"Deaktivieren Sie Autorun und Autoplay für alle Wechselmedien und externen Geräte. Nachweis: GPO-/MDM-Konfiguration.",
			"Malware-Abwehr", 2),

		// CIS 11 — Datensicherung und -wiederherstellung
		c("CIS-11.1", "Datensicherungsrichtlinie einrichten",
			"Erstellen und pflegen Sie eine Datensicherungsrichtlinie mit Häufigkeit, Aufbewahrung und Verschlüsselung (3-2-1-Regel). Nachweis: Backup-Richtlinie, Backup-Job-Konfiguration.",
			"Datensicherung", 3),
		c("CIS-11.2", "Backups durchführen",
			"Führen Sie automatisierte Backups aller kritischen Systeme und Daten durch. Nachweis: Backup-Job-Protokolle, Erfolgsquote.",
			"Datensicherung", 3),
		c("CIS-11.3", "Backups schützen",
			"Schützen Sie Backup-Daten mit Verschlüsselung und Zugriffskontrollen. Trennen Sie Backup-Daten physisch oder logisch vom Primärsystem. Nachweis: Offline-Backup-Nachweis, Verschlüsselungskonfiguration.",
			"Datensicherung", 3),
		c("CIS-11.4", "Wiederherstellung testen",
			"Testen Sie die Datenwiederherstellung mindestens vierteljährlich. Nachweis: Wiederherstellungstest-Protokoll mit Ergebnis und Datum.",
			"Datensicherung", 2),

		// CIS 12 — Verwaltung der Netzwerkinfrastruktur
		c("CIS-12.1", "Netzwerk-Infrastruktur absichern",
			"Stellen Sie sicher, dass die Netzwerk-Infrastruktur mit aktuellen Firmware-Versionen und sicheren Konfigurationen betrieben wird. Nachweis: Firmware-Inventar, Konfigurations-Baseline.",
			"Netzwerkinfrastruktur", 3),
		c("CIS-12.2", "Netzwerk-Infrastruktur-Verwaltung absichern",
			"Verwalten Sie Netzwerkgeräte über dedizierte Managementnetze oder Out-of-Band-Kanäle. Nachweis: Netzwerkplan, Verwaltungszugriffs-Konfiguration.",
			"Netzwerkinfrastruktur", 2),
		c("CIS-12.3", "Sichere Netzwerk-Konfigurationsmanagement",
			"Verwenden Sie automatisiertes Konfigurations-Management für Netzwerkgeräte. Nachweis: Änderungsprotokoll, Konfigurationsbackup.",
			"Netzwerkinfrastruktur", 2),

		// CIS 13 — Netzwerküberwachung und -verteidigung
		c("CIS-13.1", "Zentrales Netzwerk-Monitoring einrichten",
			"Stellen Sie sicher, dass der gesamte Netzwerkverkehr zentral überwacht und protokolliert wird. Nachweis: IDS/IPS-Konfiguration, SIEM-Einbindung.",
			"Netzwerküberwachung", 2),
		c("CIS-13.2", "Netzwerkdatenflüsse erfassen",
			"Erfassen Sie Netzwerkdatenflüsse (NetFlow, sFlow) zur Anomalie-Erkennung. Nachweis: Flow-Collector-Konfiguration, Analyse-Dashboard.",
			"Netzwerküberwachung", 2),
		c("CIS-13.3", "DNS-Abfragen auf Angreifer-Infrastruktur erkennen",
			"Implementieren Sie DNS-basierte Erkennungsmechanismen für Command-and-Control-Aktivitäten. Nachweis: DNS-Sicherheitskonfiguration, Alarmierungsregel.",
			"Netzwerküberwachung", 2),

		// CIS 14 — Security-Awareness und Schulungen
		c("CIS-14.1", "Schulungsprogramm für Sicherheitsbewusstsein einrichten",
			"Erstellen Sie ein dauerhaftes Security-Awareness-Programm für alle Mitarbeitenden. Nachweis: Programmbeschreibung, Schulungsplan, Teilnahmenachweise.",
			"Security Awareness", 3),
		c("CIS-14.2", "Sicherheitsbewusstsein schulen",
			"Schulen Sie alle Mitarbeitenden mindestens jährlich zu aktuellen Bedrohungen (Phishing, Passwortsicherheit, Social Engineering). Nachweis: Schulungsnachweise, Klausur-/Testergebnisse.",
			"Security Awareness", 3),
		c("CIS-14.3", "Phishing-Simulationen durchführen",
			"Führen Sie regelmäßige Phishing-Simulationen durch und nutzen Sie die Ergebnisse für gezielte Nachschulungen. Nachweis: Simulationsberichte mit Klickraten und Folgemaßnahmen.",
			"Security Awareness", 2),
		c("CIS-14.4", "Rollenspezifische Schulungen anbieten",
			"Bieten Sie zusätzliche sicherheitsbezogene Schulungen für Rollen mit erhöhtem Risikoprofil an (Admins, Entwickler, Management). Nachweis: Rollenspezifische Schulungspläne und -nachweise.",
			"Security Awareness", 2),

		// CIS 15 — Dienstleistermanagement
		c("CIS-15.1", "Inventar der Dienstleister erstellen",
			"Erstellen und pflegen Sie ein Inventar aller Drittanbieter, die Daten oder Systeme der Organisation verwalten. Nachweis: Lieferantenregister mit Risikoklassifizierung.",
			"Dienstleistermanagement", 2),
		c("CIS-15.2", "Dienstleister-Richtlinie einrichten",
			"Erstellen Sie eine Dienstleister-Sicherheitsrichtlinie mit Mindestanforderungen für alle Auftragsverarbeiter. Nachweis: Richtliniendokument, AVV-Muster.",
			"Dienstleistermanagement", 3),
		c("CIS-15.3", "Dienstleister regelmäßig überprüfen",
			"Führen Sie mindestens jährliche Sicherheitsbewertungen aller kritischen Dienstleister durch. Nachweis: Bewertungsberichte, Fragebogenrückläufe.",
			"Dienstleistermanagement", 2),

		// CIS 16 — Anwendungssoftware-Sicherheit
		c("CIS-16.1", "Anwendungssicherheitsanforderungen definieren",
			"Definieren Sie Sicherheitsanforderungen für alle selbst entwickelten und beschafften Anwendungen. Nachweis: Sicherheitsanforderungs-Dokument, Abnahme-Checkliste.",
			"Anwendungssicherheit", 2),
		c("CIS-16.2", "Sicherheitsanforderungen bei Beschaffung berücksichtigen",
			"Prüfen Sie Sicherheitsanforderungen vor der Beschaffung neuer Software und integrieren Sie diese in Verträge. Nachweis: Beschaffungs-Checkliste, Vertragsklauseln.",
			"Anwendungssicherheit", 2),
		c("CIS-16.3", "Sichere Entwicklungspraktiken anwenden",
			"Integrieren Sie sichere Entwicklungspraktiken in den SDLC (Threat Modeling, Code-Review, SAST/DAST). Nachweis: SDLC-Dokumentation, Review-Nachweise.",
			"Anwendungssicherheit", 2),

		// CIS 17 — Incident-Response-Management
		c("CIS-17.1", "Incident-Response-Programm einrichten",
			"Erstellen und pflegen Sie ein formales Incident-Response-Programm mit Richtlinie, Klassifizierungsschema und Eskalationspfaden. Nachweis: IR-Richtlinie, Prozessdokumentation.",
			"Incident Response", 3),
		c("CIS-17.2", "Incident-Response-Rollen und -Verantwortlichkeiten definieren",
			"Definieren und dokumentieren Sie klare Rollen und Verantwortlichkeiten im Incident-Response-Team. Nachweis: Teamplan, Beauftragungsschreiben, Erreichbarkeitsmatrix.",
			"Incident Response", 2),
		c("CIS-17.3", "Incident-Response-Verfahren dokumentieren",
			"Erstellen Sie dokumentierte Playbooks für häufige Vorfallstypen (Ransomware, Datenpanne, Phishing). Nachweis: Playbook-Dokumente, Testergebnis.",
			"Incident Response", 2),
		c("CIS-17.4", "Incident-Response-Übungen durchführen",
			"Führen Sie mindestens jährliche IR-Übungen (Tabletop oder Live-Test) durch. Nachweis: Übungsprotokoll mit Ergebnissen und Verbesserungsmaßnahmen.",
			"Incident Response", 2),

		// CIS 18 — Penetrationstests
		c("CIS-18.1", "Penetrationstest-Strategie einrichten",
			"Erstellen und pflegen Sie eine Penetrationstest-Strategie, die Umfang, Häufigkeit und Methodik festlegt. Nachweis: Pentest-Richtlinie, Zeitplan.",
			"Penetrationstests", 2),
		c("CIS-18.2", "Penetrationstests der Unternehmens-Infrastruktur durchführen",
			"Führen Sie mindestens jährliche externe und interne Penetrationstests durch. Nachweis: Pentest-Bericht mit Datum, Scope und Behebungsstatus.",
			"Penetrationstests", 3),
		c("CIS-18.3", "Penetrationstests von Webanwendungen durchführen",
			"Führen Sie mindestens jährliche Penetrationstests aller öffentlich zugänglichen Webanwendungen durch. Nachweis: Pentest-Bericht (OWASP-Methodik), Behebungsnachweise.",
			"Penetrationstests", 2),
	}
}
