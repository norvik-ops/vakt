// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// --- Internal helpers ---

// computeReadinessReport calculates readiness metrics given controls and evidence counts.
func ComputeReadinessReport(fw *Framework, controls []Control, evidenceCounts map[string]int) *ReadinessReport {
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
		report.ReadinessScore = ReadinessScore(report.Covered, report.Partial, report.TotalControls)
	}

	// Per-domain scores.
	for domain, total := range domainTotal {
		if total == 0 {
			continue
		}
		covered := domainCovered[domain]
		score := ReadinessScore(covered, 0, total)
		report.ByDomain = append(report.ByDomain, DomainScore{
			Domain:  domain,
			Score:   score,
			Total:   total,
			Covered: covered,
		})
	}

	return report
}

// ReadinessScore calculates a 0–100 readiness score.
// Covered controls count fully; partial controls count as half-weight.
func ReadinessScore(covered, partial, total int) float64 {
	if total == 0 {
		return 0
	}
	weighted := float64(covered) + float64(partial)*0.5
	return (weighted / float64(total)) * 100
}

// resolveStatus determines the effective status of a control.
// Priority: not_applicable > manual_status > computed from evidence.
func ResolveStatus(c Control) string {
	if c.NotApplicable {
		return "not_applicable"
	}
	if c.ManualStatus != "" {
		return c.ManualStatus
	}
	return ControlStatus(c.EvidenceCount)
}

// ControlStatus returns a computed coverage label for a control.
func ControlStatus(evidenceCount int) string {
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
func GenerateToken() (rawToken, tokenHash string, err error) {
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

// BuiltinVersion returns the canonical version string for a well-known framework name.
func BuiltinVersion(name string) string {
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
		"ISO27017":  "2015",
		"ISO27018":  "2019",
	}
	return versions[name]
}

// BuiltinControls seeds a small set of representative controls for well-known frameworks.
// variant is only used for DORA ("full" → DoraControls, "simplified" → doraSimplifiedControls).
func BuiltinControls(frameworkID, orgID, name, variant string) []Control {
	switch name {
	case "NIS2":
		return nis2Controls(frameworkID, orgID)
	case "ISO27001":
		return iso27001Controls(frameworkID, orgID)
	case "BSI":
		controls, _ := (KompendiumProvider{}).Controls(frameworkID, orgID)
		return controls
	case "CRA":
		return craControls(frameworkID, orgID)
	case "DORA":
		if variant == "simplified" {
			return doraSimplifiedControls(frameworkID, orgID)
		}
		return DoraControls(frameworkID, orgID)
	case "EUAIACT":
		return euAiActControls(frameworkID, orgID)
	case "ISO42001":
		return iso42001Controls(frameworkID, orgID)
	case "TISAX":
		return tisaxControls(frameworkID, orgID)
	case "DSGVO-TOM":
		return DsgvoTOMControls(frameworkID, orgID)
	case "CIS":
		return cisControls(frameworkID, orgID)
	case "C5":
		return c5Controls(frameworkID, orgID)
	case "KRITIS":
		return kritisControls(frameworkID, orgID)
	case "ISO27017":
		return iso27017Controls(frameworkID, orgID)
	case "ISO27018":
		return iso27018Controls(frameworkID, orgID)
	}
	return nil
}

// nis2MetaEntry holds S70-2 enrichment data for one NIS2 control.
type nis2MetaEntry struct {
	source string
	area   string
	scopes []string
}

// nis2ControlMeta maps each NIS2 control ID to its EU 2024/2690 regulation source,
// ENISA TIG V1.0 thematic area, and applicability scope (S70-2).
var nis2ControlMeta = map[string]nis2MetaEntry{
	"NIS2-A.1":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.2":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.3":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.4":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.5":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.6":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.7":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.8":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.9":  {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-A.10": {"EU 2024/2690 Art. 4", "Governance & Risikomanagement", []string{"all"}},
	"NIS2-B.1":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.2":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.3":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.4":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.5":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.6":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.7":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.8":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-B.9":  {"EU 2024/2690 Art. 8", "Incident Management", []string{"all"}},
	"NIS2-C.1":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.2":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.3":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.4":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.5":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.6":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.7":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.8":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-C.9":  {"EU 2024/2690 Art. 9", "Business Continuity", []string{"all"}},
	"NIS2-D.1":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all"}},
	"NIS2-D.2":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all"}},
	"NIS2-D.3":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all"}},
	"NIS2-D.4":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all"}},
	"NIS2-D.5":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all", "msp", "cloud"}},
	"NIS2-D.6":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all", "msp"}},
	"NIS2-D.7":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all", "msp"}},
	"NIS2-D.8":  {"EU 2024/2690 Art. 7", "Supply-Chain-Sicherheit", []string{"all"}},
	"NIS2-E.1":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.2":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.3":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.4":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.5":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.6":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.7":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-E.8":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all", "cloud", "dns"}},
	"NIS2-E.9":  {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all", "cloud", "dns"}},
	"NIS2-E.10": {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all", "cloud", "dns"}},
	"NIS2-E.11": {"EU 2024/2690 Art. 5", "Netz- & Informationssicherheit", []string{"all"}},
	"NIS2-F.1":  {"EU 2024/2690 Art. 6", "Wirksamkeitsbewertung", []string{"all"}},
	"NIS2-F.2":  {"EU 2024/2690 Art. 6", "Wirksamkeitsbewertung", []string{"all"}},
	"NIS2-F.3":  {"EU 2024/2690 Art. 6", "Wirksamkeitsbewertung", []string{"all"}},
	"NIS2-F.4":  {"EU 2024/2690 Art. 6", "Wirksamkeitsbewertung", []string{"all"}},
	"NIS2-F.5":  {"EU 2024/2690 Art. 6", "Wirksamkeitsbewertung", []string{"all"}},
	"NIS2-G.1":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.2":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.3":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.4":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.5":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.6":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.7":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.8":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-G.9":  {"EU 2024/2690 Art. 11", "Cyberhygiene & Schulungen", []string{"all"}},
	"NIS2-H.1":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-H.2":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-H.3":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-H.4":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-H.5":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-H.6":  {"EU 2024/2690 Art. 10", "Kryptographie", []string{"all"}},
	"NIS2-I.1":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.2":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.3":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.4":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.5":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.6":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.7":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.8":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.9":  {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.10": {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.11": {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.12": {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-I.13": {"EU 2024/2690 Art. 12", "HR-Sicherheit & Zugriffskontrolle", []string{"all"}},
	"NIS2-J.1":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.2":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.3":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.4":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.5":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.6":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.7":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
	"NIS2-J.8":  {"EU 2024/2690 Art. 13", "Authentifizierung & sichere Kommunikation", []string{"all"}},
}

// filterControlsByScope returns controls that match the given scope.
// Controls with applicability_scope containing "all" always pass.
// If scope is empty, all controls are returned unfiltered.
func FilterControlsByScope(controls []Control, scope string) []Control {
	if scope == "" {
		return controls
	}
	out := make([]Control, 0, len(controls))
	for _, c := range controls {
		for _, s := range c.ApplicabilityScope {
			if s == "all" || s == scope {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func nis2Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	cs := []Control{
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
	for i := range cs {
		if meta, ok := nis2ControlMeta[cs[i].ControlID]; ok {
			cs[i].RegulationSource = meta.source
			cs[i].ThematicArea = meta.area
			cs[i].ApplicabilityScope = meta.scopes
		}
	}
	return cs
}

func iso27001Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// A.5 — Organisational controls (37)
		c("A.5.1", "Richtlinien zur Informationssicherheit", "Erstelle und kommuniziere ein Set genehmigter IS-Richtlinien. Nachweis: unterschriebene, versionierte Richtliniendokumente.", "Richtlinien", "manual", 2),
		c("A.5.2", "Rollen und Verantwortlichkeiten für Informationssicherheit", "Weise IS-Rollen (ISB, CISO, Datenschutzbeauftragter) explizit zu. Nachweis: Stellenbeschreibungen, Beauftragungsschreiben.", "Organisation", "manual", 2),
		c("A.5.3", "Aufgabentrennung", "Trenne unvereinbare Aufgaben (Entwicklung/Freigabe, Buchung/Genehmigung). Nachweis: Rollenmatrix mit Trennungsnachweis.", "Organisation", "manual", 2),
		c("A.5.4", "Verantwortlichkeiten der Leitung", "Stelle sicher, dass die Leitung IS-Pflichten kommuniziert und einfordert. Nachweis: Management-Direktive, Protokolle.", "Organisation", "manual", 2),
		c("A.5.5", "Kontakt mit Behörden", "Pflege aktuelle Kontakte zu relevanten Behörden (BSI, Datenschutzbehörden, CERT). Nachweis: Kontaktliste.", "Organisation", "manual", 1),
		c("A.5.6", "Kontakt mit Interessengruppen", "Pflege Kontakte zu Fachverbänden, ISACs und Interessengruppen. Nachweis: Mitgliedschaftsnachweise, Kommunikationslog.", "Organisation", "manual", 1),
		c("A.5.7", "Threat Intelligence", "Sammle und analysiere Bedrohungsinformationen (CERT-Bund, CVE-Feeds) und integriere sie in den Risikoprozess. Nachweis: Threat-Intel-Quellen, Prozessdokumentation.", "Richtlinien", "manual", 2),
		c("A.5.8", "Informationssicherheit im Projektmanagement", "Integriere IS-Anforderungen in alle Projektprozesse. Nachweis: Projektcheckliste mit IS-Punkten.", "Organisation", "manual", 1),
		c("A.5.9", "Inventar von Informationen und zugehörigen Assets", "Führe ein vollständiges, aktuelles Asset-Register. Nachweis: Asset-Inventar mit Aktualisierungsdatum.", "Asset Management", "automated", 2),
		c("A.5.10", "Zulässige Nutzung von Informationen und zugehörigen Assets", "Dokumentiere akzeptable Nutzungsregeln für alle Asset-Klassen. Nachweis: Acceptable-Use-Policy.", "Asset Management", "manual", 1),
		c("A.5.11", "Rückgabe von Assets", "Stelle Rückgabe aller Assets bei Beschäftigungsende sicher. Nachweis: Offboarding-Checkliste.", "Asset Management", "manual", 1),
		c("A.5.12", "Klassifizierung von Informationen", "Klassifiziere alle Informationsassets nach Schutzbedarf. Nachweis: Klassifizierungsschema, Asset-Register.", "Asset Management", "manual", 2),
		c("A.5.13", "Kennzeichnung von Informationen", "Kennzeichne Informationen entsprechend ihrer Klassifizierung. Nachweis: Kennzeichnungsrichtlinie, Stichprobenprüfung.", "Asset Management", "manual", 1),
		c("A.5.14", "Informationsübertragung", "Definiere Regeln für die sichere Übertragung von Informationen (Verschlüsselung, NDA, sichere Kanäle). Nachweis: Übertragungsrichtlinie.", "Kommunikation", "manual", 2),
		c("A.5.15", "Zugangskontrolle", "Definiere Zugangskontrollrichtlinie basierend auf Geschäftsbedarf und Least-Privilege-Prinzip. Nachweis: Zugangskontrollrichtlinie.", "Zugangskontrolle", "manual", 3),
		c("A.5.16", "Identitätsmanagement", "Manage Benutzerkonten über den gesamten Lebenszyklus (Provisionierung, Review, Deprovisioning). Nachweis: IAM-Prozessdokumentation.", "Zugangskontrolle", "automated", 3),
		c("A.5.17", "Authentifizierungsinformationen", "Verwalte Passwörter und Authentifizierungsinformationen sicher (Passwortrichtlinie, Passwortmanager). Nachweis: Richtlinie, technische Konfiguration.", "Zugangskontrolle", "automated", 3),
		c("A.5.18", "Zugriffsrechte", "Genehmige und überprüfe Zugriffsrechte halbjährlich auf Aktualität. Nachweis: Review-Protokolle.", "Zugangskontrolle", "manual", 2),
		c("A.5.19", "Informationssicherheit in Lieferantenbeziehungen", "Definiere Mindest-Sicherheitsanforderungen für alle Lieferanten. Nachweis: Lieferanten-Sicherheitsrichtlinie.", "Lieferantenmanagement", "manual", 2),
		c("A.5.20", "IS in Lieferantenvereinbarungen", "Verankere IS-Anforderungen (DSGVO-AVV, Auditrechte) verbindlich in allen Lieferantenverträgen. Nachweis: Vertragsklauseln.", "Lieferantenmanagement", "manual", 3),
		c("A.5.21", "IS in der IKT-Lieferkette", "Bewerte Sicherheitsrisiken in der IKT-Lieferkette (Software-Komponenten, Cloud-Provider). Nachweis: Lieferketten-Risikobewertung, SBOM.", "Lieferantenmanagement", "manual", 2),
		c("A.5.22", "Überwachung von Lieferantendienstleistungen", "Überprüfe regelmäßig die IS-Leistung kritischer Lieferanten. Nachweis: Bewertungsberichte, Auditprotokolle.", "Lieferantenmanagement", "manual", 2),
		c("A.5.23", "IS für Cloud-Dienste", "Definiere Sicherheitsanforderungen für alle genutzten Cloud-Dienste. Nachweis: Cloud-Sicherheitsrichtlinie, Anbieter-Zertifikate.", "Lieferantenmanagement", "third_party", 3),
		c("A.5.24", "Planung und Vorbereitung des Vorfallmanagements", "Etabliere einen strukturierten Prozess zur Vorfallbehandlung. Nachweis: IR-Plan, Teambesetzungsplan.", "Vorfallmanagement", "manual", 3),
		c("A.5.25", "Bewertung und Entscheidung über IS-Ereignisse", "Stelle sicher, dass Ereignisse zeitnah klassifiziert werden. Nachweis: Klassifizierungsmatrix.", "Vorfallmanagement", "manual", 2),
		c("A.5.26", "Reaktion auf IS-Vorfälle", "Definiere konkrete Reaktionsschritte je Vorfallklasse. Nachweis: IR-Playbooks.", "Vorfallmanagement", "manual", 3),
		c("A.5.27", "Erkenntnisse aus IS-Vorfällen", "Führe Post-Incident-Reviews durch und leite Verbesserungen ab. Nachweis: Review-Berichte.", "Vorfallmanagement", "manual", 2),
		c("A.5.28", "Beweissicherung", "Definiere Verfahren zur gerichtsfesten Sicherung digitaler Beweise. Nachweis: Forensik-Checkliste.", "Vorfallmanagement", "manual", 1),
		c("A.5.29", "IS bei Störungen", "Stelle IS-Kontinuität im Krisenfall sicher. Nachweis: BCM-Plan mit IS-Komponente.", "Business Continuity", "manual", 2),
		c("A.5.30", "IKT-Bereitschaft für Business Continuity", "Stelle sicher, dass IKT-Systeme für Betriebskontinuität ausgelegt sind. Nachweis: BCM-Plan, RTO/RPO-Tabelle.", "Business Continuity", "automated", 3),
		c("A.5.31", "Gesetzliche, regulatorische und vertragliche Anforderungen", "Pflege ein Compliance-Register aller relevanten Gesetze und Verträge. Nachweis: Compliance-Register.", "Compliance", "manual", 2),
		c("A.5.32", "Rechte des geistigen Eigentums", "Stelle sicher, dass nur lizenzkonform genutzte Software eingesetzt wird. Nachweis: Software-Inventar, Lizenzübersicht.", "Compliance", "manual", 1),
		c("A.5.33", "Schutz von Aufzeichnungen", "Stelle Aufbewahrung und Schutz von Aufzeichnungen gemäß gesetzlicher Fristen sicher. Nachweis: Aufbewahrungsrichtlinie.", "Compliance", "manual", 1),
		c("A.5.34", "Datenschutz und Schutz von PII", "Stelle DSGVO-Konformität sicher. Nachweis: VVT, DSFA.", "Compliance", "manual", 3),
		c("A.5.35", "Unabhängige Überprüfung der Informationssicherheit", "Führe mindestens jährlich unabhängige IS-Audits durch. Nachweis: Auditplan, Auditberichte.", "Compliance", "manual", 2),
		c("A.5.36", "Einhaltung von IS-Richtlinien und -Standards", "Überprüfe technische Systeme auf Konformität mit IS-Richtlinien. Nachweis: Compliance-Scan-Berichte.", "Compliance", "manual", 2),
		c("A.5.37", "Dokumentierte Betriebsverfahren", "Erstelle schriftliche Betriebshandbücher für alle kritischen Systeme. Nachweis: Betriebsdokumentation.", "Betrieb", "manual", 2),

		// A.6 — People controls (8)
		c("A.6.1", "Überprüfung von Bewerbern", "Führe Hintergrundprüfungen vor der Einstellung durch. Nachweis: Screening-Richtlinie, Nachweisdokumentation.", "Personalsicherheit", "manual", 2),
		c("A.6.2", "Beschäftigungsbedingungen", "Verpflichte Mitarbeitende vertraglich auf IS- und Datenschutzpflichten. Nachweis: Arbeitsvertrag mit IS-Klauseln.", "Personalsicherheit", "manual", 2),
		c("A.6.3", "IS-Bewusstsein, -Ausbildung und -Schulung", "Schule alle Mitarbeitenden mindestens jährlich zu IS-Grundlagen. Nachweis: Schulungsnachweise, Teilnehmerlisten.", "Personalsicherheit", "manual", 3),
		c("A.6.4", "Disziplinarverfahren", "Definiere und kommuniziere Konsequenzen bei Verstößen gegen IS-Richtlinien. Nachweis: HR-Richtlinie.", "Personalsicherheit", "manual", 1),
		c("A.6.5", "Pflichten bei Beendigung des Arbeitsverhältnisses", "Stelle bei Austritt sicher, dass alle Zugänge gesperrt und Assets zurückgegeben werden. Nachweis: Offboarding-Checkliste.", "Personalsicherheit", "manual", 2),
		c("A.6.6", "Vertraulichkeitsvereinbarungen", "Stelle sicher, dass Personen mit Zugang zu sensiblen Informationen aktuelle NDAs unterzeichnet haben. Nachweis: NDA-Muster, unterzeichnete Vereinbarungen.", "Personalsicherheit", "manual", 2),
		c("A.6.7", "Fernarbeit", "Stelle sichere Arbeitsmöglichkeiten für Heimarbeitsplätze sicher. Nachweis: Telearbeitsrichtlinie, VPN-Konfiguration.", "Personalsicherheit", "manual", 2),
		c("A.6.8", "Meldung von IS-Ereignissen", "Etabliere einfache Meldekanäle für alle Mitarbeitenden. Nachweis: Meldeprozess, Kontaktinfos.", "Vorfallmanagement", "manual", 2),

		// A.7 — Physical controls (14)
		c("A.7.1", "Physische Sicherheitsbereiche", "Definiere Sicherheitsbereiche (Serverräume, Büros, RZ) und sichere sie mit physischen Barrieren. Nachweis: Raumkonzept, Zutrittskontroll-Dokumentation.", "Physische Sicherheit", "manual", 3),
		c("A.7.2", "Physische Zugangskontrolle", "Implementiere elektronische Zutrittskontrolle mit Protokollierung und Besuchermanagement. Nachweis: Zutrittssystem, Zugangsprotokolle.", "Physische Sicherheit", "manual", 3),
		c("A.7.3", "Sicherung von Büros, Räumen und Einrichtungen", "Sichere Büros physisch: abschließbare Schränke, Clean Desk. Nachweis: Begehungsprotokoll.", "Physische Sicherheit", "manual", 2),
		c("A.7.4", "Physische Sicherheitsüberwachung", "Überwache Sicherheitsbereiche durch CCTV oder Einbruchmeldung. Nachweis: CCTV-Konzept, Alarmierungskonfiguration.", "Physische Sicherheit", "automated", 2),
		c("A.7.5", "Schutz vor physischen und Umweltbedrohungen", "Schütze Systeme vor Feuer, Wasser, Strom- und Klimaausfall. Nachweis: RZ-Konzept, USV-Konfiguration, Brandschutzplan.", "Physische Sicherheit", "manual", 2),
		c("A.7.6", "Arbeiten in sicheren Bereichen", "Definiere Verhaltensregeln in sicherheitskritischen Bereichen. Nachweis: Richtlinie, Einweisung.", "Physische Sicherheit", "manual", 1),
		c("A.7.7", "Clean Desk und Clear Screen", "Setze Clean-Desk-Richtlinie und automatische Bildschirmsperren durch. Nachweis: Richtlinie, Stichprobenkontrolle.", "Physische Sicherheit", "manual", 1),
		c("A.7.8", "Aufstellung und Schutz von Betriebsmitteln", "Platziere kritische Hardware in kontrollierten Umgebungen (Klimatisierung, USV). Nachweis: RZ-Konzept.", "Physische Sicherheit", "manual", 2),
		c("A.7.9", "Sicherheit von Assets außerhalb des Unternehmens", "Definiere Regeln für den sicheren Umgang mit Assets außerhalb der Unternehmensräume. Nachweis: Mobile-Asset-Richtlinie.", "Physische Sicherheit", "manual", 2),
		c("A.7.10", "Speichermedien", "Manage physische Datenträger sicher über Lebenszyklus und Entsorgung. Nachweis: Datenträger-Policy, Vernichtungsnachweise.", "Physische Sicherheit", "manual", 2),
		c("A.7.11", "Versorgungseinrichtungen", "Stelle Redundanz bei Strom, Klima und anderen kritischen Versorgungseinrichtungen sicher. Nachweis: RZ-Konzept, USV-Testprotokoll.", "Physische Sicherheit", "automated", 2),
		c("A.7.12", "Verkabelungssicherheit", "Sichere Netzwerkverkabelung gegen unbefugten Zugriff und Beeinträchtigungen. Nachweis: Kabelplan, Sichtschutzmaßnahmen.", "Physische Sicherheit", "manual", 1),
		c("A.7.13", "Wartung von Betriebsmitteln", "Führe regelmäßige, dokumentierte Wartung aller kritischen IT-Betriebsmittel durch. Nachweis: Wartungsplan, Protokolle.", "Physische Sicherheit", "manual", 1),
		c("A.7.14", "Sichere Entsorgung von Betriebsmitteln", "Lösche Datenträger sicher vor Entsorgung (NIST 800-88, Degaussing). Nachweis: Vernichtungsnachweise.", "Physische Sicherheit", "manual", 3),

		// A.8 — Technological controls (34)
		c("A.8.1", "Benutzer-Endgeräte", "Manage Endgeräte mit MDM: Verschlüsselung, Remote-Wipe, Patch-Level. Nachweis: MDM-Konfiguration, Compliance-Bericht.", "Zugangskontrolle", "automated", 2),
		c("A.8.2", "Privilegierte Zugriffsrechte", "Verwalte Admin-Rechte restriktiv mit PAM-Lösung. Nachweis: PAM-Konfiguration, Zugriffsprotokoll.", "Zugangskontrolle", "automated", 3),
		c("A.8.3", "Einschränkung des Informationszugriffs", "Setze Least-Privilege auf Applikationsebene durch. Nachweis: Berechtigungskonzept.", "Zugangskontrolle", "automated", 2),
		c("A.8.4", "Zugriff auf Quellcode", "Beschränke Zugriff auf Source-Code auf autorisierte Entwickler. Nachweis: Repository-Zugriffskonfiguration.", "Zugangskontrolle", "automated", 2),
		c("A.8.5", "Sichere Authentifizierung", "Erzwinge MFA und sichere Login-Mechanismen. Nachweis: MFA-Konfiguration.", "Zugangskontrolle", "automated", 3),
		c("A.8.6", "Kapazitätsmanagement", "Überwache und plane Ressourcen (CPU, Speicher, Bandbreite), um Engpässe zu vermeiden. Nachweis: Monitoring-Dashboard, Kapazitätsplanung.", "Betrieb", "automated", 1),
		c("A.8.7", "Schutz vor Schadsoftware", "Implementiere Endpoint-Protection (AV/EDR) mit automatischen Updates. Nachweis: AV-Konfiguration, Scan-Berichte.", "Betrieb", "automated", 3),
		c("A.8.8", "Management technischer Schwachstellen", "Scanne regelmäßig auf Schwachstellen und behebe kritische innerhalb SLA-Fristen. Nachweis: Scanner-Berichte, Patch-Protokoll.", "Betrieb", "automated", 3),
		c("A.8.9", "Konfigurationsmanagement", "Definiere und überwache sichere Konfigurationsbaselines (CIS Benchmarks). Nachweis: Hardening-Baseline, Konfigurationsscanning-Bericht.", "Betrieb", "automated", 3),
		c("A.8.10", "Informationslöschung", "Lösche Informationen sicher, wenn nicht mehr benötigt. Nachweis: Löschkonzept, Vernichtungsnachweise.", "Betrieb", "manual", 2),
		c("A.8.11", "Datenmaskierung", "Maskiere oder pseudonymisiere personenbezogene Daten in nicht-produktiven Umgebungen. Nachweis: Maskierungskonzept, technische Umsetzung.", "Betrieb", "automated", 2),
		c("A.8.12", "Verhinderung von Datenlecks", "Implementiere DLP-Maßnahmen für kritische Daten. Nachweis: DLP-Konfiguration, Alerting-Protokoll.", "Betrieb", "automated", 2),
		c("A.8.13", "Datensicherung", "Implementiere automatisierte Backups nach 3-2-1-Prinzip. Nachweis: Backup-Job-Konfiguration, Testberichte.", "Betrieb", "automated", 3),
		c("A.8.14", "Redundanz der Informationsverarbeitungseinrichtungen", "Implementiere Redundanz für kritische Systeme (Failover, Load Balancing). Nachweis: Architektur-Diagramm.", "Betrieb", "automated", 2),
		c("A.8.15", "Protokollierung", "Protokolliere sicherheitsrelevante Ereignisse auf allen kritischen Systemen. Nachweis: Log-Konfiguration, Aufbewahrungsrichtlinie.", "Betrieb", "automated", 3),
		c("A.8.16", "Überwachungsaktivitäten", "Überwache Systeme kontinuierlich auf Anomalien (SIEM, UEBA). Nachweis: SIEM-Use-Cases, Monitoring-Dashboard.", "Betrieb", "automated", 3),
		c("A.8.17", "Zeitsynchronisation", "Synchronisiere alle Systemuhren mit autorisierten Zeitquellen (NTP). Nachweis: NTP-Konfiguration.", "Betrieb", "automated", 1),
		c("A.8.18", "Verwendung privilegierter Systemhilfsprogramme", "Kontrolliere Zugang zu privilegierten Systemwerkzeugen. Nachweis: Tool-Whitelist, Zugriffsprotokolle.", "Betrieb", "manual", 1),
		c("A.8.19", "Installation von Software auf Betriebssystemen", "Regle, welche Software auf Produktivsystemen installiert werden darf. Nachweis: Software-Whitelist, Change-Protokoll.", "Betrieb", "manual", 2),
		c("A.8.20", "Netzwerksicherheit", "Betreibe Netzwerke unter Sicherheitsgesichtspunkten (Firewall, IDS/IPS, Monitoring). Nachweis: Netzwerkplan, Firewall-Konfiguration.", "Netzwerksicherheit", "automated", 3),
		c("A.8.21", "Sicherheit von Netzdiensten", "Definiere Sicherheitsanforderungen für alle genutzten Netzdienste. Nachweis: SLA-Anforderungen.", "Netzwerksicherheit", "manual", 2),
		c("A.8.22", "Trennung von Netzen", "Segmentiere das Netzwerk nach Schutzbedarf (DMZ, Produktions-/Entwicklungsnetz). Nachweis: Netzwerkplan mit Segmentierungskonzept.", "Netzwerksicherheit", "automated", 3),
		c("A.8.23", "Web-Filterung", "Nutze Web-Proxy oder DNS-Filterung, um schädliche Websites zu blockieren. Nachweis: Filterkonfiguration.", "Netzwerksicherheit", "automated", 2),
		c("A.8.24", "Einsatz von Kryptographie", "Wende kryptographische Maßnahmen gemäß Kryptographierichtlinie an. Nachweis: Kryptographierichtlinie, KMS-Konfiguration.", "Kryptographie", "manual", 2),
		c("A.8.25", "Sicherer Entwicklungslebenszyklus", "Integriere Sicherheitsanforderungen in den SDLC (Threat Modeling, Code Review, Security Testing). Nachweis: SDLC-Dokumentation.", "Systementwicklung", "manual", 2),
		c("A.8.26", "Anforderungen an Anwendungssicherheit", "Definiere und prüfe Sicherheitsanforderungen vor Beschaffung/Entwicklung. Nachweis: Anforderungsdokument.", "Systementwicklung", "manual", 2),
		c("A.8.27", "Sichere Systemarchitektur und -entwicklungsprinzipien", "Wende sichere Architekturprinzipien an (Security by Design, Defense-in-Depth). Nachweis: Architektur-Reviews.", "Systementwicklung", "manual", 2),
		c("A.8.28", "Sichere Programmierung", "Wende sichere Programmierstandards an (OWASP Top 10, SANS CWE). Integriere SAST/DAST in CI/CD. Nachweis: Secure-Coding-Richtlinie, Scan-Berichte.", "Systementwicklung", "automated", 3),
		c("A.8.29", "Sicherheitstests in Entwicklung und Abnahme", "Führe Sicherheitstests (SAST, DAST, Pentest) vor Releases durch. Nachweis: Testberichte.", "Systementwicklung", "manual", 2),
		c("A.8.30", "Ausgelagerte Entwicklung", "Überwache Sicherheitsanforderungen bei externer Softwareentwicklung. Nachweis: Vertragsklauseln, Code-Review-Nachweis.", "Systementwicklung", "manual", 2),
		c("A.8.31", "Trennung von Entwicklungs-, Test- und Produktionsumgebungen", "Trenne Entwicklungs-, Test- und Produktionsumgebungen strikt. Nachweis: Umgebungskonzept, Zugriffsprotokolle.", "Systementwicklung", "automated", 2),
		c("A.8.32", "Änderungsmanagement", "Stelle sicher, dass alle Änderungen geplant, genehmigt und dokumentiert werden. Nachweis: Change-Tickets.", "Betrieb", "manual", 2),
		c("A.8.33", "Testinformationen", "Schütze und kontrolliere die Verwendung von Testdaten. Nachweis: Test-Datenrichtlinie, Maskierungsnachweis.", "Systementwicklung", "manual", 1),
		c("A.8.34", "Schutz von Informationssystemen bei Audittests", "Koordiniere Audittests und schütze Produktivsysteme vor unbeabsichtigten Auswirkungen. Nachweis: Audit-Test-Plan, Genehmigungsnachweis.", "Compliance", "manual", 1),
	}
}

// craControls returns controls for the EU Cyber Resilience Act (CRA, 2024).
// Applies to manufacturers of products with digital elements sold in the EU.
// 23 controls: CRA-1.x (10) Annex I Part I, CRA-2.x (3) Annex I Part II vuln handling,
// CRA-3.x (6) Annex I Part II technical, CRA-4.x (4) Annex II user information.
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

// DoraControls returns controls for DORA — Digital Operational Resilience Act (EU 2022/2554).
// Applies to financial entities (banks, insurers, investment firms, fintechs) and their ICT providers.
// doraSimplifiedControls returns the 15 controls for the DORA Simplified ICT Risk Framework
// defined in RTS EU 2024/1774 Chapter II (Art. 3–10), applicable to "small and non-interconnected"
// financial entities under DORA Art. 16.
func doraSimplifiedControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// Art. 3 — Allgemeine Anforderungen
		c("DORA-S.1", "Organisatorischer Rahmen und Governance (vereinfacht)",
			"Stelle sicher, dass die Geschäftsleitung die Gesamtverantwortung für den IKT-Risikomanagementrahmen trägt und die internen Zuständigkeiten klar geregelt sind. Nachweis: Governance-Dokument, Beschluss der Geschäftsführung, Aufgabenverteilung.",
			"Governance & Organisation", "manual", 3),
		c("DORA-S.2", "IKT-Risikobewertung (vereinfacht)",
			"Führe mindestens jährlich eine vereinfachte Bewertung der IKT-Risiken durch, die alle kritischen Systeme und Abhängigkeiten berücksichtigt. Nachweis: Risikobewertungsdokument mit Datum, Risikoeigentümer, Bewertungsmatrix.",
			"Governance & Organisation", "manual", 3),
		c("DORA-S.3", "IKT-Asset-Inventar (vereinfacht)",
			"Führe ein aktuelles Inventar aller IKT-Assets, die für die Erbringung der Finanzdienstleistung wesentlich sind. Nachweis: Asset-Liste mit Klassifizierung (kritisch/nicht-kritisch), letzte Aktualisierung.",
			"Governance & Organisation", "manual", 2),
		// Art. 4 — IKT-Risikomanagement-Policy
		c("DORA-S.4", "IKT-Risikomanagement-Policy (vereinfacht)",
			"Erstelle und halte eine dokumentierte Policy für das IKT-Risikomanagement, die Schutzziele, Risikotoleranz und Verantwortlichkeiten definiert. Nachweis: Policy-Dokument mit Genehmigungsdatum, Versionierung.",
			"Risikomanagement-Policy", "manual", 3),
		c("DORA-S.5", "IKT-Sicherheitsleitlinie",
			"Dokumentiere Sicherheitsanforderungen für den Betrieb kritischer IKT-Systeme (Zugriffskontrollen, Verschlüsselung, Netzwerksicherheit). Nachweis: Sicherheitsleitlinie, technische Konfigurationsstandards.",
			"Risikomanagement-Policy", "manual", 2),
		// Art. 5 — Schutz und Erkennung
		c("DORA-S.6", "Technische Schutzmaßnahmen (vereinfacht)",
			"Implementiere grundlegende technische Sicherheitskontrollen: Firewalls, Zugriffskontrolle, Patch-Management, Antiviren-/EDR-Schutz. Nachweis: Maßnahmenkatalog, Konfigurationsnachweis, Patch-Protokoll.",
			"Schutz & Erkennung", "manual", 3),
		c("DORA-S.7", "Erkennung von IKT-Anomalien (vereinfacht)",
			"Implementiere angemessene Mechanismen zur Erkennung von Anomalien und IKT-Vorfällen (z.B. Logging, Alerting, einfaches SIEM). Nachweis: Logging-Konfiguration, Alert-Regeln, Eskalationsprozess.",
			"Schutz & Erkennung", "automated", 2),
		// Art. 6 — Reaktion und Wiederherstellung
		c("DORA-S.8", "Reaktionsplan für IKT-Vorfälle (vereinfacht)",
			"Definiere einen einfachen Reaktionsplan für IKT-Vorfälle mit klaren Eskalationspfaden, Kommunikationskanälen und Zuständigkeiten. Nachweis: Incident-Response-Plan, Kontaktliste, Testablauf.",
			"Reaktion & Wiederherstellung", "manual", 3),
		c("DORA-S.9", "Backup und Wiederherstellung (vereinfacht)",
			"Stelle regelmäßige Backups kritischer Daten und Systeme sicher und verifiziere die Wiederherstellungsfähigkeit durch Tests. Nachweis: Backup-Policy, Backup-Protokoll, Restore-Test-Ergebnis mit Datum.",
			"Reaktion & Wiederherstellung", "automated", 3),
		c("DORA-S.10", "Business-Continuity-Plan (vereinfacht)",
			"Erstelle einen einfachen Business-Continuity-Plan für kritische IKT-Systeme mit RTO/RPO-Vorgaben. Nachweis: BCP-Dokument, RTO/RPO-Tabelle, letzte Testübung.",
			"Reaktion & Wiederherstellung", "manual", 2),
		// Art. 7 — Tests
		c("DORA-S.11", "IKT-Resilienztests (vereinfacht)",
			"Führe jährlich angemessene Tests der IKT-Resilienz durch, z.B. Vulnerability Scans oder einfache Penetrationstests. TLPT ist für den vereinfachten Rahmen nicht verpflichtend. Nachweis: Testplan, Bericht mit Datum, Maßnahmenverfolgung.",
			"Tests", "manual", 2),
		// Art. 8 — Drittparteien-IKT-Risiko (vereinfacht)
		c("DORA-S.12", "Drittparteien-IKT-Risiko (vereinfacht)",
			"Identifiziere und bewerte die IKT-Risiken durch Drittanbieter. Für den vereinfachten Rahmen ist kein vollständiges CTPP-Register erforderlich. Nachweis: Liste kritischer IKT-Drittanbieter, Risikobewertung, wesentliche Vertragsklauseln.",
			"Drittparteienrisiken", "manual", 2),
		c("DORA-S.13", "Ausstiegsstrategie für kritische IKT-Drittanbieter (vereinfacht)",
			"Definiere für jeden kritischen IKT-Drittanbieter eine grundlegende Ausstiegsstrategie, um Abhängigkeitsrisiken zu begrenzen. Nachweis: Exit-Plan-Dokument oder -Abschnitt im Drittparteienregister.",
			"Drittparteienrisiken", "manual", 1),
		// Art. 9 — Meldeverfahren
		c("DORA-S.14", "Meldeverfahren für schwerwiegende IKT-Vorfälle (vereinfacht)",
			"Stelle sicher, dass schwerwiegende IKT-Vorfälle gemäß DORA Art. 19–20 fristgerecht an die zuständige Behörde (BaFin) gemeldet werden. Nachweis: Meldetemplate, Klassifizierungsschema, Meldungsarchiv.",
			"Incident-Meldung", "manual", 3),
		// Art. 10 — Berichterstattung an Leitungsorgan
		c("DORA-S.15", "Berichterstattung an das Leitungsorgan",
			"Berichte mindestens jährlich an das Leitungsorgan über den Stand des IKT-Risikomanagements, aufgetretene Vorfälle und Verbesserungsmaßnahmen. Nachweis: Berichtsdokument mit Datum, Sitzungsprotokoll.",
			"Governance & Organisation", "manual", 2),
	}
}

func DoraControls(frameworkID, orgID string) []Control {
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

// DoraISO27001Mapping maps each DORA control code to the corresponding ISO 27001:2022 Annex A clauses.
var DoraISO27001Mapping = map[string]string{
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
		// Art. 26 — Betreiberpflichten (Deployer Obligations)
		// Für Unternehmen, die Hochrisiko-KI-Systeme EINSETZEN (nicht entwickeln).
		// Die meisten DACH-KMU sind Betreiber, nicht Anbieter.
		c("AIACT-12.1", "Verwendungszweck-Compliance als Betreiber (Art. 26 Abs. 1)",
			"Stelle sicher, dass Hochrisiko-KI-Systeme ausschließlich gemäß der Gebrauchsanweisung des Anbieters eingesetzt werden. Führe ein Verzeichnis aller eingesetzten Hochrisiko-KI-Systeme. Nachweis: KI-System-Register, Nutzungsrichtlinie, Schulungsnachweise für Betreiberpersonal.",
			"Betreiberpflichten", "manual", 3),
		c("AIACT-12.2", "Menschliche Aufsicht als Betreiber (Art. 26 Abs. 2)",
			"Weise geeignete qualifizierte Personen zur menschlichen Aufsicht über Hochrisiko-KI-Systeme zu. Dokumentiere Qualifikationen und Zuständigkeiten. Führe Schulungsnachweise. Nachweis: Stellenbeschreibung, Schulungszertifikate, Aufsichtsprotokoll.",
			"Betreiberpflichten", "manual", 3),
		c("AIACT-12.3", "Grundrechte-Folgenabschätzung als Betreiber (Art. 26 Abs. 9 / Art. 27)",
			"Führe eine Grundrechte-Folgenabschätzung (GRFA) vor dem erstmaligen Einsatz von Hochrisiko-KI-Systemen durch, die öffentliche Dienstleistungen erbringen oder Entscheidungen über Personen treffen. Nachweis: GRFA-Dokumentation, Genehmigungsprotokoll, Registrierungsnachweis (EU-Datenbank Art. 49).",
			"Betreiberpflichten", "manual", 3),
		c("AIACT-12.4", "Datenschutz-Impact-Assessment für KI (Art. 26 Abs. 8 / DSGVO Art. 35)",
			"Führe bei Hochrisiko-KI-Systemen mit biometrischen, personenbezogenen oder entscheidungsrelevanten Daten eine DSGVO-DPIA durch. Koordiniere mit dem Datenschutzbeauftragten. Nachweis: DPIA-Bericht, DSB-Konsultationsprotokoll, Risikoregister.",
			"Betreiberpflichten", "manual", 3),
		c("AIACT-12.5", "Protokollierung und Monitoring als Betreiber (Art. 26 Abs. 5–6)",
			"Speichere die vom KI-System generierten Protokolle für den gesetzlichen Mindestzeitraum (6 Monate oder nach sektorrechtlichen Anforderungen). Melde schwerwiegende Vorfälle und Fehlfunktionen an den Anbieter. Nachweis: Log-Archiv, Aufbewahrungsrichtlinie, Vorfallsmeldungsverfahren.",
			"Betreiberpflichten", "automated", 2),
		c("AIACT-12.6", "Information der betroffenen Personen (Art. 26 Abs. 11)",
			"Informiere betroffene natürliche Personen transparent darüber, dass sie einer automatisierten Entscheidung durch ein Hochrisiko-KI-System unterliegen, soweit nicht durch Unions- oder nationales Recht ausgeschlossen. Nachweis: Datenschutzhinweis, Informationspflicht-Dokumentation.",
			"Betreiberpflichten", "manual", 2),
		// Art. 50 — Transparenzpflichten für General-Purpose AI Interaktion
		c("AIACT-13.1", "Kennzeichnung KI-generierter Inhalte (Art. 50 Abs. 2)",
			"Kennzeichne alle durch KI erzeugten Text-, Bild-, Audio- und Videoinhalte als KI-generiert (maschinenlesbar oder sichtbar), sofern die Inhalte nicht offensichtlich künstlich sind. Gilt ab August 2026. Nachweis: Implementierungsnachweis, Wasserzeichen-/Metadaten-Dokumentation, Prozessbeschreibung.",
			"KI-Transparenz", "automated", 2),
		c("AIACT-13.2", "Offenlegung KI-Interaktion (Art. 50 Abs. 1)",
			"Informiere Nutzer klar und erkennbar, wenn sie mit einem KI-System (Chatbot, virtuellem Assistenten) interagieren, sofern dies nicht offensichtlich ist. Ausnahme: strafrechtliche Verfolgung. Nachweis: UX-Dokumentation, Datenschutzhinweis, technischer Nachweis der Offenlegung.",
			"KI-Transparenz", "manual", 2),
		c("AIACT-13.3", "Deepfake-Offenlegungspflicht (Art. 50 Abs. 4)",
			"Beim Einsatz von KI für synthetische Medien (Deepfakes), die Personen zeigen oder Ereignisse darstellen, die nicht stattgefunden haben: Offenlegungspflicht in maschinenlesbarem Format. Gilt für journalistische, künstlerische und Satire-Ausnahmen eingeschränkt. Nachweis: Policy, technische Implementierung.",
			"KI-Transparenz", "manual", 1),
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

func DsgvoTOMControls(frameworkID, orgID string) []Control {
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

// c5Controls returns controls for BSI Cloud Computing Compliance Criteria Catalogue C5:2026.
// Source: BSI C5:2026, veröffentlicht 07.04.2026, 168 Sicherheitskriterien + 6 General Conditions.
// Kriterien-Codes entsprechen direkt dem BSI-Dokument (GC-01…PSS-12).
func c5Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: "C5-" + id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// ── GC: Allgemeine Bedingungen (General Conditions — informativ, nicht prüfpflichtig) ──
		c("GC-01", "Anwendbares Recht und Rechtsraum", "CSP dokumentiert anwendbares Recht, Gerichtsstand, Länder und Zonen. Nachweis: AGB, DPA, Standort-Dokumentation.", "Allgemeine Bedingungen", "document", 1),
		c("GC-02", "Verfügbarkeit und Störungshandling im Normalbetrieb", "CSP beschreibt SLAs, Wartungsfenster und Incident-Handling im Normalbetrieb. Nachweis: SLA-Dokument, Incident-Report-Vorlagen.", "Allgemeine Bedingungen", "document", 1),
		c("GC-03", "Wiederherstellungsparameter im Notbetrieb", "CSP veröffentlicht RTO/RPO-Werte und Notbetriebs-Parameter. Nachweis: Verfügbarkeitsgarantien, BCP-Auszüge.", "Allgemeine Bedingungen", "document", 1),
		c("GC-04", "Ansatz zur Sicherstellung der Dienstverfügbarkeit", "CSP beschreibt Redundanzarchitektur und Verfügbarkeitsmaßnahmen. Nachweis: Architektur-Beschreibung, Verfügbarkeitsstatistiken.", "Allgemeine Bedingungen", "document", 1),
		c("GC-05", "Umgang mit Ermittlungsanfragen von Behörden", "CSP legt dar, wie Behördenanfragen behandelt werden. Nachweis: Transparenzbericht, Richtlinie zu Behördenanfragen.", "Allgemeine Bedingungen", "document", 1),
		c("GC-06", "Zertifizierungen und Testate", "CSP nennt aktuelle Zertifizierungen (ISO 27001, SOC 2, C5-Testat). Nachweis: Gültige Zertifikate oder Testatsberichte.", "Allgemeine Bedingungen", "document", 2),

		// ── OIS: Organisation der Informationssicherheit ──
		c("OIS-01", "Informationssicherheitsmanagementsystem (ISMS)", "CSP betreibt ein ISMS nach anerkanntem Standard (ISO 27001 oder vergleichbar). Nachweis: ISMS-Scope, ISO-27001-Zertifikat oder C5-Testat.", "Organisation der IS", "document", 3),
		c("OIS-02", "Informationssicherheitsrichtlinie", "Leitungsorgan verabschiedet IS-Richtlinie mit Zielen, Geltungsbereich, Verantwortlichkeiten. Nachweis: Unterzeichnete Richtlinie, Revisionshistorie.", "Organisation der IS", "manual", 2),
		c("OIS-03", "Schnittstellen und Abhängigkeiten", "CSP dokumentiert Abhängigkeiten zu Unterauftragnehmern und internen Schnittstellen. Nachweis: Schnittstellendokumentation, Dienstleister-Verzeichnis.", "Organisation der IS", "document", 2),
		c("OIS-04", "Aufgabentrennung", "Kritische Aufgaben sind durch Rollentrennung abgesichert (z.B. Entwicklung/Betrieb, Revisor/Entwickler). Nachweis: Rollenmatrix, Zugriffsdokumentation.", "Organisation der IS", "manual", 2),
		c("OIS-05", "Bedrohungsanalyse", "CSP betreibt strukturierte Auswertung von Bedrohungsinformationen (Threat Intelligence). Nachweis: TI-Feeds, Auswertungsberichte, Prozessbeschreibung.", "Organisation der IS", "manual", 2),
		c("OIS-06", "Kontakt zu Behörden und Interessengruppen", "CSP pflegt Kontakt zu relevanten Behörden (BSI, CERT-Bund) und Branchen-CERTs. Nachweis: Kontaktlisten, Meldeverfahrens-Dokumentation.", "Organisation der IS", "document", 1),
		c("OIS-07", "Risikomanagement-Policy", "CSP hat eine dokumentierte Risikomanagement-Policy mit Risikokriterien und -appetit. Nachweis: Policy-Dokument, Genehmigung durch Leitungsorgan.", "Organisation der IS", "manual", 3),
		c("OIS-08", "Anwendung der Risikomanagement-Policy — Risikoanalyse", "CSP führt regelmäßige Risikoanalysen für Cloud-Dienste durch. Nachweis: Risikoregister, Bewertungsberichte mit Datum.", "Organisation der IS", "manual", 3),
		c("OIS-09", "Anwendung der Risikomanagement-Policy — Risikobehandlung", "Identifizierte Risiken werden behandelt (Mitigierung, Akzeptanz, Transfer). Nachweis: Risikobehandlungsplan, Maßnahmennachweise.", "Organisation der IS", "manual", 3),
		c("OIS-10", "Informationssicherheit im Projektmanagement", "IS-Anforderungen sind in den Projektentwicklungs-Prozess integriert. Nachweis: Projekt-Templates mit IS-Checkliste, Abnahmedokumentation.", "Organisation der IS", "manual", 2),

		// ── SP: Sicherheitsrichtlinien und -verfahren ──
		c("SP-01", "Dokumentation, Kommunikation und Bereitstellung von Richtlinien", "Richtlinien sind aktuell, zugänglich und an alle relevanten Mitarbeitenden kommuniziert. Nachweis: Richtlinienverzeichnis, Zugriffsnachweise, Lesungsbestätigungen.", "Sicherheitsrichtlinien", "document", 2),
		c("SP-02", "Überprüfung und Genehmigung von Richtlinien", "Richtlinien werden mindestens jährlich reviewed und von autorisierten Stellen genehmigt. Nachweis: Review-Protokolle, Genehmigungsunterschriften.", "Sicherheitsrichtlinien", "manual", 2),
		c("SP-03", "Ausnahmen von bestehenden Richtlinien", "Ausnahmen werden formal beantragt, genehmigt, dokumentiert und zeitlich befristet. Nachweis: Ausnahmen-Register, Genehmigungen mit Ablaufdatum.", "Sicherheitsrichtlinien", "manual", 2),

		// ── HR: Personal ──
		c("HR-01", "Überprüfung von Qualifikation und Vertrauenswürdigkeit", "Vor Einstellung wird die Eignung (Referenzen, ggf. polizeiliches Führungszeugnis) für sicherheitssensible Stellen geprüft. Nachweis: Screening-Prozess, Checklisten.", "Personal", "manual", 2),
		c("HR-02", "Beschäftigungsbedingungen", "Mitarbeitende unterzeichnen Vertraulichkeits- und IS-Verpflichtungen. Nachweis: Unterzeichnete Verträge, Vertraulichkeitserklärungen.", "Personal", "document", 2),
		c("HR-03", "Schulungs- und Sensibilisierungsprogramm für IS", "Regelmäßige IS-Schulungen für alle Mitarbeitenden (mind. jährlich). Nachweis: Schulungsplan, Teilnehmerlisten, Abschlussnachweise.", "Personal", "manual", 2),
		c("HR-04", "Disziplinarmaßnahmen", "Verfahren für IS-Verstöße sind definiert und bekannt. Nachweis: Disziplinarrichtlinie, Eskalationsverfahren.", "Personal", "document", 1),
		c("HR-05", "Verantwortlichkeiten bei Beschäftigungsende oder -wechsel", "Rückgabe von Assets und Entzug von Zugriffsrechten bei Austritt ist geregelt. Nachweis: Offboarding-Checkliste, Zugriffsprotokoll.", "Personal", "manual", 2),
		c("HR-06", "Vertraulichkeitsvereinbarungen", "NDAs mit Mitarbeitenden, Subunternehmern und externen Parteien. Nachweis: Unterzeichnete NDAs.", "Personal", "document", 2),
		c("HR-07", "Telearbeit — Richtlinie", "Richtlinie für sicheres Arbeiten außerhalb des Unternehmensgeländes. Nachweis: Remote-Work-Policy, Sicherheitsanforderungen.", "Personal", "document", 1),
		c("HR-08", "Telearbeit — Umsetzung", "Technische und organisatorische Maßnahmen für sicheres Remote-Working (VPN, verschlüsselte Geräte). Nachweis: Konfigurationsnachweise, VPN-Protokolle.", "Personal", "automated", 2),

		// ── AM: Asset Management ──
		c("AM-01", "Asset-Management-Rahmen", "Prozess für Erfassung, Klassifizierung und Verwaltung von Assets ist dokumentiert. Nachweis: Asset-Management-Policy, Prozessbeschreibung.", "Asset Management", "document", 2),
		c("AM-02", "Asset-Inventar", "Vollständiges Inventar aller informationsverarbeitenden Assets inkl. Eigentümer. Nachweis: Asset-Inventar (aktuell, mit Datum), Prüfbericht.", "Asset Management", "automated", 3),
		c("AM-03", "Hardware-Asset-Inventar", "Physische IT-Assets sind vollständig erfasst mit Standort und Lebenszyklus-Status. Nachweis: Hardware-Inventar, CMDB-Auszug.", "Asset Management", "automated", 2),
		c("AM-04", "Software-Asset-Inventar", "Installierte Software ist vollständig erfasst inkl. Versionsstand. Nachweis: Software-Inventar (CMDB), automatisierte Scan-Ergebnisse.", "Asset Management", "automated", 2),
		c("AM-05", "Richtlinie für ordnungsgemäße und sichere Nutzung von Assets", "Akzeptable Nutzungsrichtlinie für alle Asset-Typen. Nachweis: Acceptable-Use-Policy, Schulungsnachweise.", "Asset Management", "document", 1),
		c("AM-06", "Inbetriebnahme von Hardware", "Neue Hardware wird vor Inbetriebnahme auf Sicherheits-Konfiguration geprüft und inventarisiert. Nachweis: Inbetriebnahme-Checkliste, Konfigurationsnachweise.", "Asset Management", "manual", 2),
		c("AM-07", "Außerbetriebnahme von Hardware", "Hardware wird sicher außer Betrieb genommen inkl. Datenlöschung/Vernichtung. Nachweis: Außerbetriebnahme-Protokoll, Vernichtungszertifikat.", "Asset Management", "manual", 2),
		c("AM-08", "Ordnungsgemäße Nutzung, sichere Handhabung und Rückgabe von Assets", "Mitarbeitende bestätigen Nutzungsregeln und geben Assets bei Austritt zurück. Nachweis: Übergabeprotokolle, Rückgabebestätigungen.", "Asset Management", "manual", 2),
		c("AM-09", "Asset-Klassifizierung und -Kennzeichnung", "Informationsassets sind nach Schutzbedarf klassifiziert und entsprechend gekennzeichnet. Nachweis: Klassifizierungsschema, Kennzeichnungsnachweise.", "Asset Management", "manual", 2),
		c("AM-10", "Schutz von Hardware im Wartezustand", "Außer Betrieb gestellte oder wartende Hardware ist physisch gesichert. Nachweis: Inventar für Assets im Wartezustand, Zugangsprotokolle.", "Asset Management", "manual", 1),
		c("AM-11", "Transport von Hardware", "Physischer Transport von Hardware wird sicher durchgeführt (Versiegelung, Protokollierung). Nachweis: Transport-Protokolle, Kuriervereinbarungen.", "Asset Management", "document", 1),
		c("AM-12", "Wechselmedien und Endgeräte", "Wechselmedien (USB, Festplatten) werden kontrolliert und gesichert eingesetzt. Nachweis: Wechselmedia-Policy, Verschlüsselungsnachweise, Inventar.", "Asset Management", "manual", 2),

		// ── PS: Physische Sicherheit ──
		c("PS-01", "Anforderungen an physische Sicherheit und Umweltkontrolle", "Sicherheitsanforderungen für Rechenzentren und Büros sind definiert und umgesetzt. Nachweis: Sicherheitsrichtlinie, Begehungsberichte.", "Physische Sicherheit", "manual", 3),
		c("PS-02", "Redundanzmodell", "Dokumentiertes Redundanzmodell für Strom, Kühlung und Konnektivität. Nachweis: Architekturdiagramme, SLA-Belegung, Testberichte.", "Physische Sicherheit", "document", 3),
		c("PS-03", "Perimeterschutz", "Physische Absicherung des Rechenzentrums (Zäune, Schleusen, Türsicherungen). Nachweis: Begehungsprotokoll, Fotos/Videos, Sicherheitskonzept.", "Physische Sicherheit", "manual", 3),
		c("PS-04", "Physische Zugangskontrolle", "Nur autorisierte Personen erhalten Zutritt zu Rechenzentren (Ausweisleser, PIN, Biometrie). Nachweis: Zugangsprotokoll, Zutrittsregeln, Review-Nachweise.", "Physische Sicherheit", "manual", 3),
		c("PS-05", "Schutz vor externen Bedrohungen und Umwelteinflüssen", "Maßnahmen gegen Feuer, Wasser, Extremwetter und andere Umweltrisiken. Nachweis: Feuerschutzanlage, Wasserleckage-Sensoren, Klimakonzept.", "Physische Sicherheit", "manual", 2),
		c("PS-06", "Schutz vor Stromausfällen und ähnlichen Risiken", "USV und Notstromversorgung (Diesel-Generator) für kritische Systeme. Nachweis: USV-Protokolle, Generator-Testberichte, SLA.", "Physische Sicherheit", "automated", 3),
		c("PS-07", "Überwachung von Betriebs- und Umgebungsparametern", "Kontinuierliche Überwachung von Temperatur, Feuchtigkeit, Strom und Sicherheit. Nachweis: Monitoring-Dashboard, Alert-Konfiguration, Eventlogs.", "Physische Sicherheit", "automated", 2),
		c("PS-08", "Anforderungen an die Arbeitplatzsicherheit", "Sicherheitsanforderungen für Büroarbeitsplätze (Clean Desk, Bildschirmsperre). Nachweis: Clean-Desk-Policy, Stichproben-Protokolle.", "Physische Sicherheit", "manual", 1),

		// ── OPS: Betrieb ──
		c("OPS-01", "Kapazitätsmanagement — Planung", "Kapazitätsplanung für Rechen-, Speicher- und Netzwerkressourcen. Nachweis: Kapazitätsplan, Wachstumsprognosen.", "Betrieb", "document", 2),
		c("OPS-02", "Kapazitätsmanagement — Monitoring", "Automatisiertes Monitoring von Ressourcenauslastung mit Alerting. Nachweis: Monitoring-Konfiguration, Auslastungsberichte, Alert-Protokolle.", "Betrieb", "automated", 2),
		c("OPS-03", "Kapazitätsmanagement — Steuerung", "Prozess zur Ressourcenskalierung bei Kapazitätsengpässen. Nachweis: Eskalationsprozess, Skalierungsprotokoll.", "Betrieb", "manual", 1),
		c("OPS-04", "Schutz vor Schadsoftware — Richtlinien und Verfahren", "Richtlinie zur Malware-Prävention und -Behandlung. Nachweis: Anti-Malware-Policy, Prozessbeschreibung.", "Betrieb", "document", 3),
		c("OPS-05", "Schutz vor Schadsoftware — Implementierung", "EDR/AV-Lösung auf allen relevanten Systemen. Nachweis: Deployment-Nachweis, Scan-Berichte, Signatur-Aktualitätsnachweise.", "Betrieb", "automated", 3),
		c("OPS-06", "Datensicherung und -wiederherstellung — Richtlinien", "Backup-Richtlinie mit RPO/RTO und Aufbewahrungsfristen. Nachweis: Backup-Policy, RPO/RTO-Dokumentation.", "Betrieb", "document", 3),
		c("OPS-07", "Datensicherung — Monitoring", "Automatisierte Überwachung von Backup-Jobs mit Alerting bei Fehlern. Nachweis: Monitoring-Dashboard, Backup-Fehler-Protokolle.", "Betrieb", "automated", 2),
		c("OPS-08", "Datensicherung — Regelmäßige Tests", "Restore-Tests mindestens vierteljährlich mit Ergebnisdokumentation. Nachweis: Restore-Test-Protokolle mit Datum und Ergebnis.", "Betrieb", "manual", 3),
		c("OPS-09", "Datensicherung — Speicherung", "Backups werden getrennt vom Produktionssystem und verschlüsselt gespeichert. Nachweis: Backup-Speicherort-Dokumentation, Verschlüsselungsnachweise.", "Betrieb", "manual", 2),
		c("OPS-10", "Protokollierung und Monitoring — Richtlinien", "Logging-Richtlinie mit Anforderungen an Protokolltypen, Aufbewahrung und Schutz. Nachweis: Logging-Policy, Protokollklassifizierung.", "Betrieb", "document", 2),
		c("OPS-11", "Protokollierung — Richtlinie für Cloud-Dienst-Daten", "Richtlinie zu Logging von Kundendaten (wann, was, wie lange). Nachweis: Datenschutzfreundliche Logging-Policy, Kundendokumentation.", "Betrieb", "document", 2),
		c("OPS-12", "Protokollierung — Zugriff, Aufbewahrung und Löschung", "Logs sind vor unbefugtem Zugriff geschützt, Aufbewahrungsfristen dokumentiert. Nachweis: Zugriffskontrollen für Logs, Retention-Policy.", "Betrieb", "manual", 2),
		c("OPS-13", "Protokollierung — SIEM", "SIEM-System zur Korrelation und Analyse von Sicherheitsereignissen. Nachweis: SIEM-Konfiguration, Use-Case-Dokumentation, Alert-Regeln.", "Betrieb", "automated", 3),
		c("OPS-14", "Protokollierung — Aufbewahrungsdauer", "Logs werden mindestens 12 Monate (sicherheitsrelevante: 24 Monate) aufbewahrt. Nachweis: Retention-Konfiguration, Nachweis der Aufbewahrungsdauer.", "Betrieb", "automated", 2),
		c("OPS-15", "Protokollierung — Nachvollziehbarkeit", "Aktionen von privilegierten Nutzern sind vollständig protokolliert und nachvollziehbar. Nachweis: Admin-Aktions-Logs, Integritätsnachweise.", "Betrieb", "automated", 3),
		c("OPS-16", "Protokollierung — Konfiguration", "Log-Quellen, -Felder und -Formate sind standardisiert konfiguriert. Nachweis: Logging-Konfiguration, Standardformat-Dokumentation.", "Betrieb", "automated", 2),
		c("OPS-17", "Protokollierung — Verfügbarkeit der Monitoring-Software", "Monitoring-Infrastruktur ist hochverfügbar (redundant, ausfallsicher). Nachweis: Redundanzkonzept, Verfügbarkeitsstatistiken.", "Betrieb", "automated", 2),
		c("OPS-18", "Schwachstellenmanagement — Richtlinien", "Richtlinie für Identifikation, Bewertung und Behebung von Schwachstellen. Nachweis: Vulnerability-Management-Policy, SLAs für Behebungsfristen.", "Betrieb", "document", 3),
		c("OPS-19", "Störungsmanagement — Richtlinien und Verfahren", "Incident-Management-Prozess mit Klassifizierung und Eskalation. Nachweis: Incident-Management-Richtlinie, Eskalationspfade.", "Betrieb", "document", 3),
		c("OPS-20", "Störungsmanagement — Umsetzung", "Incidents werden gemäß definiertem Prozess bearbeitet, dokumentiert und abgeschlossen. Nachweis: Incident-Tickets, Post-Mortem-Berichte.", "Betrieb", "manual", 3),
		c("OPS-21", "Ausfallmanagement — Umsetzung", "Kritische Systemausfälle werden nach dokumentiertem Verfahren behandelt. Nachweis: Runbook/Playbook, Notfallprotokolle.", "Betrieb", "manual", 3),
		c("OPS-22", "Penetrationstests", "Regelmäßige Penetrationstests (mind. jährlich). Nachweis: Pentest-Bericht, Scope-Dokumentation, Behebungsnachweise.", "Betrieb", "manual", 3),
		c("OPS-23", "Messungen, Analysen und Assessments von Schwachstellen/Vorfällen", "Regelmäßige Auswertung von Schwachstellen- und Vorfallsdaten für Verbesserungen. Nachweis: Kennzahlenberichte, Trend-Analysen.", "Betrieb", "manual", 2),
		c("OPS-24", "Einbeziehung von Cloud-Service-Kunden bei Vorfällen", "Kunden werden bei sicherheitsrelevanten Vorfällen zeitnah informiert. Nachweis: Kommunikationsrichtlinie, Muster-Benachrichtigungen, Vorfallsprotokolle.", "Betrieb", "manual", 2),
		c("OPS-25", "Schwachstellenscans", "Regelmäßige automatisierte Schwachstellenscans aller exponierten Systeme. Nachweis: Scan-Konfiguration, Scan-Berichte, Eskalationsprotokoll.", "Betrieb", "automated", 3),
		c("OPS-26", "System-Hardening", "Systeme werden nach Sicherheits-Baselines (CIS, BSI) gehärtet. Nachweis: Hardening-Checklisten, Compliance-Scan-Ergebnisse.", "Betrieb", "automated", 3),
		c("OPS-27", "Patch-Management — Richtlinien", "Patch-Management-Richtlinie mit Fristen nach Kritikalität (kritisch ≤7 Tage). Nachweis: Patch-Policy, SLA-Tabelle.", "Betrieb", "document", 3),
		c("OPS-28", "Patch-Management — Umsetzung", "Patches werden gemäß Richtlinie eingespielt und Compliance überwacht. Nachweis: Patch-Berichte, Compliance-Dashboard, Ausnahmen-Register.", "Betrieb", "automated", 3),
		c("OPS-29", "Extern bezogene Komponenten", "Software-Drittkomponenten werden auf Schwachstellen überwacht (SCA, SBOM). Nachweis: SBOM, SCA-Scan-Berichte, Update-Protokolle.", "Betrieb", "automated", 2),
		c("OPS-30", "Datentrennung — Richtlinien", "Richtlinie zur logischen Trennung von Kundendaten (Multi-Tenancy). Nachweis: Trennungskonzept, Architektur-Dokumentation.", "Betrieb", "document", 3),
		c("OPS-31", "Datentrennung — Umsetzung", "Technische Maßnahmen zur Datentrennung sind implementiert und verifiziert. Nachweis: Konfigurationsnachweise, Isolationstest-Ergebnisse.", "Betrieb", "automated", 3),
		c("OPS-32", "Confidential Computing — Richtlinien", "Richtlinie für Einsatz von Confidential Computing (TEE). Nachweis: Policy-Dokument, Anwendungsbereich.", "Betrieb", "document", 2),
		c("OPS-33", "Confidential Computing — Remote Attestation", "Remote-Attestation-Mechanismus für TEE-basierte Workloads. Nachweis: Attestierungs-Protokolle, Konfigurationsnachweise.", "Betrieb", "automated", 2),
		c("OPS-34", "Container-Management — Richtlinien", "Richtlinie für sicheres Container-Management (Images, Registry, Runtime). Nachweis: Container-Sicherheits-Policy, Image-Scan-Strategie.", "Betrieb", "document", 2),
		c("OPS-35", "Container-Management — Umsetzung", "Sichere Container-Images (gescannt, signiert), Laufzeit-Policies (PSP/Admission). Nachweis: Image-Scan-Berichte, Kubernetes-Policies, Runtime-Security-Konfiguration.", "Betrieb", "automated", 2),

		// ── IAM: Identitäts- und Zugriffsverwaltung ──
		c("IAM-01", "Richtlinie für Identitäten und Zugriffsrechte", "Umfassende Zugriffsrichtlinie (least privilege, need-to-know). Nachweis: IAM-Policy, Rollenkonzept.", "Identitäts- und Zugriffsverwaltung", "document", 3),
		c("IAM-02", "Vergabe und Änderung von Identitäten und Zugriffsrechten", "Zugriffsrechte werden nach formalen Genehmigungsverfahren vergeben und geändert. Nachweis: Ticket-System, Genehmigungsprotokolle.", "Identitäts- und Zugriffsverwaltung", "manual", 3),
		c("IAM-03", "Risikobasiertes Verfahren für Sperrung und Entzug von Identitäten", "Risikobasiertes Verfahren für sofortige Sperrung bei Verdacht oder Austritt. Nachweis: Sperrprotokoll, automatisierter Offboarding-Prozess.", "Identitäts- und Zugriffsverwaltung", "automated", 3),
		c("IAM-04", "Entzug oder Anpassung von Zugriffsrechten bei Aufgabenwechsel", "Rechte werden bei Rollenwechsel zeitnah entzogen/angepasst. Nachweis: Mover-Checkliste, Review-Protokoll.", "Identitäts- und Zugriffsverwaltung", "manual", 2),
		c("IAM-05", "Regelmäßige Überprüfung von Zugriffsrechten", "Access Reviews (mind. halbjährlich) für alle Benutzerkonten. Nachweis: Access-Review-Berichte, Korrekturnachweise.", "Identitäts- und Zugriffsverwaltung", "manual", 3),
		c("IAM-06", "Privilegierte Zugriffsrechte", "Privilegierte Konten werden separat verwaltet (PAM), nur temporär vergeben. Nachweis: PAM-Konfiguration, Just-in-Time-Zugriffs-Protokolle.", "Identitäts- und Zugriffsverwaltung", "automated", 3),
		c("IAM-07", "Zugriff auf Cloud-Kundendaten", "Zugriff auf Kundendaten durch CSP-Mitarbeitende ist protokolliert und auf das Minimum beschränkt. Nachweis: Zugriffsprotokoll, Break-Glass-Verfahren, Kundeninformation.", "Identitäts- und Zugriffsverwaltung", "automated", 3),
		c("IAM-08", "Authentifizierungsmechanismen", "Starke Authentifizierung (MFA) für alle privilegierten und Remote-Zugänge. Nachweis: MFA-Konfiguration, Ausnahmen-Register.", "Identitäts- und Zugriffsverwaltung", "automated", 3),
		c("IAM-09", "Vertraulichkeit von Authentifizierungsinformationen", "Passwörter werden verschlüsselt gespeichert (bcrypt/Argon2), nie im Klartext. Nachweis: Passwort-Hashing-Konfiguration, Code-Review.", "Identitäts- und Zugriffsverwaltung", "automated", 2),

		// ── CRY: Kryptographie und Schlüsselmanagement ──
		c("CRY-01", "Richtlinie für den Einsatz kryptografischer Mechanismen", "Kryptografie-Policy mit zugelassenen Algorithmen und Mindestschlüssellängen. Nachweis: Crypto-Policy, Algorithmen-Liste.", "Kryptographie", "document", 3),
		c("CRY-02", "Kryptografisches Change-Management", "Verfahren für den Wechsel kryptografischer Algorithmen bei Schwachstellen. Nachweis: Change-Prozess, Crypto-Agility-Konzept.", "Kryptographie", "manual", 2),
		c("CRY-03", "Überprüfung von Kryptografiepraktiken", "Regelmäßige Überprüfung eingesetzter Kryptografie auf Aktualität. Nachweis: Review-Berichte, Algorithmen-Inventar.", "Kryptographie", "manual", 2),
		c("CRY-04", "Schutz von Daten bei Übertragung (Transportverschlüsselung)", "TLS 1.2/1.3 für alle externen Übertragungen, kein veraltetes SSL/TLS. Nachweis: TLS-Konfiguration, Scan-Ergebnisse (z.B. SSL Labs).", "Kryptographie", "automated", 3),
		c("CRY-05", "Verschlüsselung sensibler Daten at-Rest", "Sensible Daten werden at-rest verschlüsselt (AES-256). Nachweis: Verschlüsselungskonfiguration, Storage-Dokumentation.", "Kryptographie", "automated", 3),
		c("CRY-06", "Sichere Schlüsselgenerierung", "Kryptografische Schlüssel werden sicher generiert (CSPRNG, HSM). Nachweis: Schlüsselgenerierungs-Verfahren, HSM-Konfiguration.", "Kryptographie", "manual", 2),
		c("CRY-07", "Rotation kryptografischer Schlüssel", "Schlüssel werden nach definierter Lebensdauer rotiert. Nachweis: Rotation-Policy, Schlüssel-Inventar mit Ablaufdaten.", "Kryptographie", "manual", 3),
		c("CRY-08", "Ausstellung von Public-Key-Zertifikaten", "Zertifikate werden von vertrauenswürdigen CAs ausgestellt, Revokation ist möglich. Nachweis: Zertifikats-Inventar, CA-Dokumentation.", "Kryptographie", "automated", 2),
		c("CRY-09", "Sichere Schlüsselbereitstellung", "Schlüssel werden sicher provisioniert (kein Klartextübertragung). Nachweis: Key-Provisioning-Prozess, HSM-Integration.", "Kryptographie", "manual", 2),
		c("CRY-10", "Sichere Schlüsselaufbewahrung", "Schlüssel werden in HSM oder dediziertem KMS aufbewahrt, nie im Code. Nachweis: KMS-Konfiguration, Code-Review.", "Kryptographie", "automated", 3),
		c("CRY-11", "Kryptografische Schlüsselarchivierung", "Archivierung von Schlüsseln für verschlüsselte Langzeitspeicherung. Nachweis: Archivierungskonzept, Archiv-Zugangsprotokolle.", "Kryptographie", "manual", 1),
		c("CRY-12", "Kryptografisches Schlüsseltransitions-Management", "Prozess für sicheren Übergang zu neuen Schlüsseln bei Rotation. Nachweis: Transitionsplan, Test-Ergebnisse.", "Kryptographie", "manual", 2),
		c("CRY-13", "Umgang mit kompromittierten Schlüsseln", "Verfahren für sofortige Revokation und Schlüsselaustausch bei Kompromittierung. Nachweis: Incident-Playbook für Schlüssel-Kompromittierung.", "Kryptographie", "manual", 3),
		c("CRY-14", "Sichere Deaktivierung kryptografischer Schlüssel", "Außer-Dienst-Stellung von Schlüsseln ist dokumentiert und sicher. Nachweis: Deaktivierungs-Protokoll.", "Kryptographie", "manual", 1),
		c("CRY-15", "Anforderungen an Pre-Shared Keys", "PSKs sind ausreichend lang und zufällig, werden sicher geteilt. Nachweis: PSK-Generierungsverfahren, Sicherheitsanforderungen.", "Kryptographie", "manual", 1),
		c("CRY-16", "Betriebskontinuität für das Schlüsselmanagement", "Schlüsselmanagement-Systeme sind hochverfügbar (Redundanz, Failover). Nachweis: Redundanzkonzept, Failover-Test.", "Kryptographie", "automated", 2),
		c("CRY-17", "Kryptografischer Schlüssel-Lebenszyklus-Management", "Vollständiger Schlüssel-Lebenszyklus (Generierung→Rotation→Archivierung→Zerstörung) ist dokumentiert. Nachweis: KMS-Konfiguration, Lifecycle-Policy.", "Kryptographie", "automated", 3),
		c("CRY-18", "Nutzung externer Schlüsselverwaltungssysteme", "Externe KMS-Integration (AWS KMS, Azure Key Vault) ist dokumentiert und sicher konfiguriert. Nachweis: Integration-Dokumentation, Zugriffsprotokolle.", "Kryptographie", "automated", 2),
		c("CRY-19", "Sicherer Umgang mit kundenverwalteten Schlüsseln (BYOK/HYOK)", "BYOK/HYOK-Mechanismus ist sicher implementiert und dokumentiert. Nachweis: BYOK-Architektur, Schlüsseltransferdokumentation.", "Kryptographie", "manual", 2),

		// ── COS: Kommunikationssicherheit ──
		c("COS-01", "Technische Schutzmaßnahmen", "Netzwerk-Sicherheitsmaßnahmen (Firewall, IDS/IPS) sind implementiert. Nachweis: Firewall-Regelwerke, IDS-Konfiguration.", "Kommunikationssicherheit", "automated", 3),
		c("COS-02", "Sicherheitsanforderungen für Verbindungen im CSP-Netzwerk", "Interne Netzwerkverbindungen sind nach definiertem Sicherheitsstandard gesichert. Nachweis: Netzwerk-Sicherheitsrichtlinie, Konfigurationsnachweise.", "Kommunikationssicherheit", "automated", 2),
		c("COS-03", "Monitoring von Verbindungen im CSP-Netzwerk", "Netzwerkverbindungen werden kontinuierlich auf Anomalien überwacht. Nachweis: NDR/NTA-Konfiguration, Alert-Protokolle.", "Kommunikationssicherheit", "automated", 2),
		c("COS-04", "Netzwerkübergreifender Zugriff", "Zugriffe über Netzwerkgrenzen hinweg sind kontrolliert (DMZ, Proxy). Nachweis: Netzwerktopologie, Firewall-Regeln.", "Kommunikationssicherheit", "manual", 2),
		c("COS-05", "Netzwerke für Verwaltung", "Management-Netzwerke sind von Produktionsnetzwerken getrennt. Nachweis: Netzwerksegmentierungsdokumentation, VLAN-Konfiguration.", "Kommunikationssicherheit", "automated", 3),
		c("COS-06", "Trennung des Datenverkehrs in gemeinsam genutzten Netzumgebungen", "Multi-Tenant-Netzwerktrennung verhindert Datenlecks zwischen Kunden. Nachweis: VLAN/VxLAN-Konfiguration, Isolationstest-Ergebnisse.", "Kommunikationssicherheit", "automated", 3),
		c("COS-07", "Dokumentation der Netzwerktopologie", "Aktuelle Netzwerktopologie-Dokumentation inkl. Segmentierung und Datenflüsse. Nachweis: Netzwerkdiagramme, Datenflusskarte.", "Kommunikationssicherheit", "document", 2),
		c("COS-08", "Richtlinien für die Datenübertragung", "Richtlinien für sichere Datenübertragung (intern und extern). Nachweis: Data-Transfer-Policy, Verschlüsselungsanforderungen.", "Kommunikationssicherheit", "document", 2),

		// ── PI: Portabilität und Interoperabilität ──
		c("PI-01", "Sicherheit von Ein- und Ausgabeschnittstellen", "Schnittstellen für Datenmigration/-export sind abgesichert (Auth, Verschlüsselung). Nachweis: API-Sicherheitsdokumentation, Auth-Konfiguration.", "Portabilität", "manual", 2),
		c("PI-02", "Vertragliche Vereinbarungen zur Datenbereitstellung", "Vertragliche Regelungen für Datenportabilität und Übergabe bei Vertragsende. Nachweis: SLA/DPA-Klauseln zu Datenportabilität, Exit-Prozess.", "Portabilität", "document", 2),
		c("PI-03", "Sichere Löschung von Daten", "Kundendaten werden bei Vertragsende sicher und nachweislich gelöscht. Nachweis: Löschzertifikat, Löschprozess-Dokumentation.", "Portabilität", "manual", 3),

		// ── DEV: Beschaffung, Entwicklung und Änderung von Systemen ──
		c("DEV-01", "Richtlinien für Entwicklung/Beschaffung von Systemkomponenten", "Sicherheitsanforderungen sind Teil des Entwicklungs-/Beschaffungsprozesses (SSDLC). Nachweis: SSDLC-Policy, Sicherheitsanforderungskatalog.", "Entwicklung & Änderung", "document", 3),
		c("DEV-02", "Auslagerung der Entwicklung", "Ausgelagerte Entwicklung unterliegt gleichen Sicherheitsanforderungen. Nachweis: Entwickler-Verträge mit Sicherheitsklauseln, Audit-Recht.", "Entwicklung & Änderung", "document", 2),
		c("DEV-03", "Richtlinien für Änderungen an Systemkomponenten", "Change-Management-Richtlinie für alle sicherheitsrelevanten Systemänderungen. Nachweis: Change-Policy, Genehmigungsverfahren.", "Entwicklung & Änderung", "document", 2),
		c("DEV-04", "Schulung zu CI/CD-Sicherheit", "Entwickler werden in sicherer Continuous Delivery geschult. Nachweis: Schulungsunterlagen, Teilnehmerlisten.", "Entwicklung & Änderung", "manual", 1),
		c("DEV-05", "Designdokumentation für Sicherheitsfunktionen", "Sicherheitsrelevante Design-Entscheidungen sind dokumentiert. Nachweis: Security-Design-Dokument, Threat-Model.", "Entwicklung & Änderung", "document", 2),
		c("DEV-06", "Risikobewertung und Priorisierung von Änderungen", "Änderungen werden nach Sicherheitsrisiko bewertet und priorisiert. Nachweis: Change-Risk-Assessment, Priorisierungsmatrix.", "Entwicklung & Änderung", "manual", 2),
		c("DEV-07", "Tests von Änderungen", "Sicherheitstests sind Teil des Änderungsprozesses (SAST, DAST, Review). Nachweis: Test-Berichte, Security-Gate-Ergebnisse.", "Entwicklung & Änderung", "automated", 3),
		c("DEV-08", "Protokollierung von Änderungen", "Alle Änderungen werden in einem Änderungsprotokoll (Audit Trail) erfasst. Nachweis: Change-Log, Versionskontrolle.", "Entwicklung & Änderung", "automated", 2),
		c("DEV-09", "Versionskontrolle", "Quellcode und Konfigurationen werden in einer Versionskontrolle (Git) verwaltet. Nachweis: Repository-Konfiguration, Branch-Schutz-Regeln.", "Entwicklung & Änderung", "automated", 2),
		c("DEV-10", "Freigabe in der Produktionsumgebung", "Deployments in Produktion durchlaufen formalen Genehmigungsprozess. Nachweis: Deployment-Genehmigungsprotokolle, 4-Augen-Prinzip.", "Entwicklung & Änderung", "manual", 2),
		c("DEV-11", "Schutz von Entwicklungs- und Testumgebungen", "Dev/Test-Umgebungen sind vom Produktionsbetrieb getrennt und gesichert. Nachweis: Umgebungskonzept, Zugriffskontrollen.", "Entwicklung & Änderung", "manual", 2),
		c("DEV-12", "Trennung von Umgebungen", "Strikte Trennung zwischen Entwicklungs-, Test- und Produktionsumgebungen. Nachweis: Umgebungsarchitektur, Netzwerktrennung.", "Entwicklung & Änderung", "automated", 3),
		c("DEV-13", "Transparenz über Software-Komponenten", "SBOM (Software Bill of Materials) für alle eingesetzten Komponenten. Nachweis: SBOM (SPDX oder CycloneDX), Aktualisierungshistorie.", "Entwicklung & Änderung", "automated", 2),
		c("DEV-14", "Sicherer Einsatz von Fremd-Hardware und -Software", "Drittkomponenten werden auf Integrität und Sicherheit geprüft. Nachweis: Komponenten-Prüfungsprotokoll, Supply-Chain-Sicherheitskonzept.", "Entwicklung & Änderung", "manual", 2),
		c("DEV-15", "Ausnahmen vom Change-Management-Prozess", "Notfall-Changes sind geregelt und werden nachträglich dokumentiert. Nachweis: Emergency-Change-Richtlinie, Notfall-Änderungsprotokoll.", "Entwicklung & Änderung", "manual", 1),

		// ── SSO: Steuerung und Überwachung von Dienstleistern ──
		c("SSO-01", "Richtlinien und Verfahren zur Steuerung von Dienstleistern", "Richtlinie für Auswahl, Beauftragung und Überwachung von Unterauftragnehmern. Nachweis: Third-Party-Management-Policy.", "Dienstleister-Steuerung", "document", 3),
		c("SSO-02", "Risikobewertung von Dienstleistern", "Dienstleister werden vor Beauftragung und regelmäßig hinsichtlich IS-Risiken bewertet. Nachweis: Vendor-Risk-Assessment, Bewertungsberichte.", "Dienstleister-Steuerung", "manual", 3),
		c("SSO-03", "Datenverarbeitung durch Dienstleister", "AVVs mit allen datenverarbeitenden Unterauftragnehmern. Nachweis: AVV-Verzeichnis, unterzeichnete AVVs.", "Dienstleister-Steuerung", "document", 3),
		c("SSO-04", "Verzeichnis der Dienstleister", "Aktuelles Verzeichnis aller wesentlichen Unterauftragnehmer. Nachweis: Unterauftragnehmer-Verzeichnis (aktuell, mit Rollen).", "Dienstleister-Steuerung", "document", 2),
		c("SSO-05", "Monitoring der Anforderungserfüllung durch Dienstleister", "Regelmäßige Überprüfung der Compliance-Einhaltung durch Dienstleister (Audits, Zertifikate). Nachweis: Audit-Berichte, Zertifikats-Nachweise.", "Dienstleister-Steuerung", "manual", 2),
		c("SSO-06", "Vertragliche Kündigungs-/Ausstiegsstrategie für Dienstleister", "Ausstiegsstrategie für kritische Dienstleister ist dokumentiert. Nachweis: Exit-Strategie, Vertragskündigungsklauseln.", "Dienstleister-Steuerung", "document", 2),
		c("SSO-07", "Sicherstellung von Transparenz innerhalb von Dienstleistern", "Unterauftragnehmer informieren über ihre eigenen Unterauftragnehmer. Nachweis: Sub-Subunternehmerliste, Vertragsklauseln zur Weitergabe.", "Dienstleister-Steuerung", "document", 1),
		c("SSO-08", "Kontrolle des Austauschs mit Funktionskomponenten-Lieferanten", "Schnittstellen zu Software-Komponentenlieferanten (z.B. OSS) sind kontrolliert. Nachweis: OSS-Policy, Komponenten-Review-Prozess.", "Dienstleister-Steuerung", "manual", 1),

		// ── SIM: Security Incident Management ──
		c("SIM-01", "Richtlinie für das Sicherheitsvorfallsmanagement", "Incident-Management-Richtlinie mit Klassifizierung, Eskalation und Kommunikation. Nachweis: Incident-Response-Policy.", "Sicherheitsvorfallsmanagement", "document", 3),
		c("SIM-02", "Sicherheitsvorfalls-Reaktionspläne", "Dokumentierte Incident-Response-Pläne für relevante Vorfallstypen. Nachweis: IR-Playbooks, Runbooks.", "Sicherheitsvorfallsmanagement", "document", 3),
		c("SIM-03", "Bearbeitung von Sicherheitsvorfällen", "Vorfälle werden gemäß Prozess bearbeitet, dokumentiert und abgeschlossen. Nachweis: Incident-Tickets, Timeline-Dokumentation.", "Sicherheitsvorfallsmanagement", "manual", 3),
		c("SIM-04", "Dokumentation und Reporting von Sicherheitsvorfällen", "Vorfälle werden vollständig dokumentiert und an relevante Stakeholder berichtet. Nachweis: Vorfallsdokumentation, Berichtsvorlagen, Post-Mortem.", "Sicherheitsvorfallsmanagement", "manual", 2),
		c("SIM-05", "Meldepflicht des Personals", "Mitarbeitende sind verpflichtet, Sicherheitsereignisse zu melden. Nachweis: Schulungsnachweis, Eskalationskontakt-Dokumentation.", "Sicherheitsvorfallsmanagement", "manual", 2),
		c("SIM-06", "Auswertungs- und Lernprozess", "Lessons Learned nach Vorfällen fließen in Verbesserungen ein. Nachweis: Post-Mortem-Protokolle, Maßnahmenverfolgung.", "Sicherheitsvorfallsmanagement", "manual", 2),

		// ── BCM: Business Continuity Management ──
		c("BCM-01", "Business-Continuity- und Notfallmanagementsystem", "Dokumentiertes BCM-System mit Scope, Strategie und Verantwortlichkeiten. Nachweis: BCM-Policy, BIA, BCM-Rahmenwerk.", "Business Continuity Management", "document", 3),
		c("BCM-02", "Business-Impact-Analyse (BIA)", "Regelmäßige BIA identifiziert kritische Dienste und Abhängigkeiten. Nachweis: BIA-Bericht (aktuell ≤12 Monate), RTO/RPO-Tabelle.", "Business Continuity Management", "manual", 3),
		c("BCM-03", "Business-Continuity-Pläne", "Dokumentierte BCPs für alle kritischen Dienste. Nachweis: BCP-Dokumente, Wiederherstellungs-Prozeduren.", "Business Continuity Management", "document", 3),
		c("BCM-04", "Tests der Business Continuity", "BCM-Tests (Tabletop, Full-DR) mindestens jährlich. Nachweis: Test-Berichte mit Datum, Ergebnis und Verbesserungsmaßnahmen.", "Business Continuity Management", "manual", 3),

		// ── COM: Compliance ──
		c("COM-01", "Identifikation anwendbarer Anforderungen", "Alle rechtlichen, regulatorischen und vertraglichen Anforderungen sind erfasst. Nachweis: Compliance-Register, Gesetzgebungsübersicht.", "Compliance", "document", 2),
		c("COM-02", "Richtlinie für Planung und Durchführung von Audits", "Interne Audit-Richtlinie mit Planung, Unabhängigkeit und Berichterstattung. Nachweis: Audit-Policy, Auditplan.", "Compliance", "document", 2),
		c("COM-03", "Interne Audits des ISMS", "Mindestens jährliche interne ISMS-Audits durch unabhängige Auditoren. Nachweis: Audit-Berichte, Maßnahmenverfolgung.", "Compliance", "manual", 3),
		c("COM-04", "Informationen zur IS-Performance und Management-Assessment", "Regelmäßiges Management-Review der IS-Kennzahlen und ISMS-Leistung. Nachweis: Management-Review-Protokoll, KPI-Dashboard.", "Compliance", "manual", 2),

		// ── INQ: Umgang mit behördlichen Ermittlungsanfragen ──
		c("INQ-01", "Rechtliche Bewertung von Ermittlungsanfragen", "Behördenanfragen werden rechtlich geprüft, bevor Daten herausgegeben werden. Nachweis: Richtlinie, Rechtsgutachten.", "Behördenanfragen", "manual", 2),
		c("INQ-02", "Information der Cloud-Kunden über Ermittlungsanfragen", "Kunden werden über Behördenanfragen informiert (soweit rechtlich zulässig). Nachweis: Benachrichtigungsrichtlinie, Transparenzbericht.", "Behördenanfragen", "document", 2),
		c("INQ-03", "Begrenzung des Zugriffs auf Daten bei Ermittlungsanfragen", "Datenzugriff durch Behörden wird auf das rechtlich Notwendige beschränkt. Nachweis: Zugriffsprotokoll, Rechtsgrundlagen-Dokumentation.", "Behördenanfragen", "manual", 2),
		c("INQ-04", "Kommunikation technischer Offenlegungsverfahren", "CSP kommuniziert technische Verfahren für Datenzugriffe durch Behörden. Nachweis: Technische Dokumentation, Kundenkommunikation.", "Behördenanfragen", "document", 1),

		// ── PSS: Produktsicherheit für Cloud-Kundschaft ──
		c("PSS-01", "Empfehlungen für Cloud-Kundschaft", "CSP stellt Sicherheitsleitfäden und Konfigurationsempfehlungen für Kunden bereit. Nachweis: Security-Hardening-Guide, Kundendokumentation.", "Produktsicherheit für Kunden", "document", 2),
		c("PSS-02", "Identifikation von Schwachstellen im Cloud-Dienst", "Prozess zur Identifikation und Behebung von Schwachstellen in der Kundenoberfläche. Nachweis: Vulnerability-Disclosure-Policy, CVE-Tracking.", "Produktsicherheit für Kunden", "automated", 3),
		c("PSS-03", "Information der Kunden über bekannte Schwachstellen", "Kunden werden zeitnah über sicherheitsrelevante Schwachstellen informiert. Nachweis: Security-Advisories, Kundenbenachrichtigungen.", "Produktsicherheit für Kunden", "manual", 2),
		c("PSS-04", "Fehlerbehandlung und Logging-Mechanismen", "Applikation behandelt Fehler sicher (kein Sensitive Data Exposure). Nachweis: Code-Review, SAST-Berichte, Error-Handling-Policy.", "Produktsicherheit für Kunden", "automated", 2),
		c("PSS-05", "Authentifizierungsmechanismen (Kundenebene)", "Starke Authentifizierung für Kundenportale und APIs (MFA-Unterstützung). Nachweis: Auth-Konfiguration, MFA-Dokumentation.", "Produktsicherheit für Kunden", "automated", 3),
		c("PSS-06", "Session-Management", "Sichere Session-Verwaltung (Timeout, Invalidierung, Token-Rotation). Nachweis: Session-Konfiguration, OWASP-Checkliste.", "Produktsicherheit für Kunden", "automated", 2),
		c("PSS-07", "Vertraulichkeit von Authentifizierungsinformationen (Kundenebene)", "Kunden-Passwörter werden verschlüsselt gespeichert, nie im Klartext. Nachweis: Hashing-Konfiguration, Sicherheitsarchitektur.", "Produktsicherheit für Kunden", "automated", 2),
		c("PSS-08", "Rollen- und Rechterahmen", "Rollenbasiertes Zugriffsmodell (RBAC) für Kunden-Tenants. Nachweis: RBAC-Konzept, Rechte-Matrix, API-Dokumentation.", "Produktsicherheit für Kunden", "manual", 2),
		c("PSS-09", "Autorisierungsmechanismen", "Zugriffskontrolle auf API- und Anwendungsebene (AuthZ). Nachweis: AuthZ-Konfiguration, Tests.", "Produktsicherheit für Kunden", "automated", 3),
		c("PSS-10", "Software-Defined Networking", "SDN-Komponenten sind sicher konfiguriert und abgehärtet. Nachweis: SDN-Konfiguration, Security-Policy.", "Produktsicherheit für Kunden", "automated", 2),
		c("PSS-11", "Images für virtuelle Maschinen und Container", "VM/Container-Images sind gehärtet, gescannt und signiert. Nachweis: Image-Scan-Berichte, Signierungs-Konfiguration, Base-Image-Policy.", "Produktsicherheit für Kunden", "automated", 3),
		c("PSS-12", "Region der Datenverarbeitung und -speicherung", "Datenverarbeitungs- und Speicherorte sind dokumentiert und vertraglich festgelegt. Nachweis: Datenhaltungskonzept, DPA, Region-Dokumentation.", "Produktsicherheit für Kunden", "document", 2),
	}
}

// kritisControls returns controls for KRITIS-Dachgesetz (KRITISDachG).
// Source: Dachgesetz zur Stärkung der physischen Resilienz kritischer Anlagen,
// in Kraft getreten 11. März 2026 (BGBl. 2026 I Nr. 66).
// Bezieht sich auf §§ 8, 12, 13, 16, 18, 20 KRITIS-DachG.
func kritisControls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: "KRITIS-" + id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// ── Registrierungspflichten (§§ 8, 9) ──
		c("DG.1", "Registrierung beim BSI (§8 Abs.1)", "Betreiber kritischer Anlagen registrieren sich innerhalb der Frist beim BSI und melden relevante Anlagen. Nachweis: BSI-Registrierungsnachweis, Registrierungsnummer.", "Registrierung & Meldepflichten", "document", 3),
		c("DG.2", "Aktualisierung der Registrierungsdaten (§8 Abs.6)", "Registrierungsdaten werden bei Änderungen zeitnah aktualisiert. Nachweis: Aktualisierungsprotokoll, BSI-Bestätigung.", "Registrierung & Meldepflichten", "document", 2),

		// ── Risikoanalyse (§12) ──
		c("DG.3", "Risikoanalyse und Risikobewertung durch Betreiber (§12 Abs.1)", "Betreiber führen eine systematische Risikoanalyse für die kritische Anlage durch, die Bedrohungsszenarien, Abhängigkeiten und Schutzmaßnahmen umfasst. Nachweis: Risikoanalysedokument (≤12 Monate), Methodik, Eigentümer.", "Risikoanalyse", "manual", 3),

		// ── Resilienzmaßnahmen (§13) ──
		c("DG.4", "Maßnahmen zur Gewährleistung der Resilienz (§13 Abs.1)", "Betreiber implementieren angemessene technische, sicherheitsbezogene und organisatorische Maßnahmen zum Schutz der kritischen Anlage und ihrer Dienste. Nachweis: Maßnahmenplan, Umsetzungsnachweise.", "Resilienzmaßnahmen", "manual", 3),
		c("DG.5", "Verhältnismäßigkeit und Stand der Technik (§13 Abs.2)", "Maßnahmen entsprechen dem Stand der Technik und sind verhältnismäßig (Kosten-Nutzen-Analyse). Nachweis: Technologiebewertung, Vergleich mit Branchenstandards.", "Resilienzmaßnahmen", "manual", 2),

		// ── §13 Abs.3 Nr.1 — Notfallvorsorge ──
		c("DG.6", "Notfallvorsorge (§13 Abs.3 Nr.1)", "Maßnahmen zur Verhütung von Vorfällen (präventive Maßnahmen, Notfallvorsorgeplan). Nachweis: Notfallvorsorgeplan, präventive Maßnahmenübersicht.", "Notfallvorsorge", "document", 2),

		// ── §13 Abs.3 Nr.2 — Physische Sicherheit ──
		c("DG.7", "Physischer Schutz — Bauliche und technische Maßnahmen (§13 Abs.3 Nr.2a)", "Strukturelle und technische Absicherung des Perimeters (Zäune, Sicherheitsglas, Tore). Nachweis: Sicherheitskonzept, Begehungsprotokoll, Fotos.", "Physische Sicherheit", "manual", 3),
		c("DG.8", "Physischer Schutz — Umgebungsüberwachung (§13 Abs.3 Nr.2b)", "Instrumente zur Überwachung von Umgebungsparametern (Temperatur, Feuer, Wasser, Erschütterung). Nachweis: Sensorliste, Alarmkonfiguration, Testberichte.", "Physische Sicherheit", "automated", 2),
		c("DG.9", "Physischer Schutz — Detektionseinrichtungen (§13 Abs.3 Nr.2c)", "Einbruchmeldesysteme, Bewegungsmelder und Videoüberwachung. Nachweis: Anlagendokumentation, Wartungsberichte, Alarmprotokoll.", "Physische Sicherheit", "automated", 3),
		c("DG.10", "Physischer Schutz — Zutrittskontrolle (§13 Abs.3 Nr.2d)", "Zutrittskontrollsystem mit Autorisierungsmanagement und Protokollierung. Nachweis: Zutrittskontrollkonzept, Zugangsprotokoll, Review-Nachweis.", "Physische Sicherheit", "manual", 3),

		// ── §13 Abs.3 Nr.3 — Krisenmanagement ──
		c("DG.11", "Risiko- und Krisenmanagement (§13 Abs.3 Nr.3a)", "Dokumentiertes Risiko- und Krisenmanagement-Verfahren mit Eskalationspfaden. Nachweis: Krisenmanagement-Konzept, Eskalationsmatrix.", "Krisenmanagement", "document", 3),
		c("DG.12", "Alarmierungsverfahren (§13 Abs.3 Nr.3b)", "Vorab definierte Alarmierungs- und Eskalationsverfahren für Vorfälle. Nachweis: Alarmierungsplan, Kontaktliste (aktuell), Test-Protokoll.", "Krisenmanagement", "manual", 3),

		// ── §13 Abs.3 Nr.4 — Business Continuity ──
		c("DG.13", "Aufrechterhaltung des Betriebs (§13 Abs.3 Nr.4a)", "Maßnahmen zur Aufrechterhaltung des Betriebs (Notstrom, redundante Systeme, Ersatzteillager). Nachweis: BCM-Plan, Redundanzkonzept, Notstrom-Testprotokoll.", "Business Continuity", "manual", 3),
		c("DG.14", "Alternative Lieferketten (§13 Abs.3 Nr.4b)", "Identifikation alternativer Lieferketten für kritische Ressourcen. Nachweis: Lieferkettenanalyse, Notlieferanten-Verzeichnis.", "Business Continuity", "document", 2),

		// ── §13 Abs.3 Nr.5 — Personalsicherheit ──
		c("DG.15", "Personal- und Dienstleistersicherheit (§13 Abs.3 Nr.5)", "Sicherheitsanforderungen für Mitarbeitende und externe Dienstleister mit Zugang zur kritischen Anlage (Überprüfung, Einweisung, NDAs). Nachweis: Personalsicherheits-Richtlinie, Einweisungsnachweise, Vertraulichkeitsverpflichtungen.", "Personalsicherheit", "manual", 2),

		// ── §13 Abs.3 Nr.6 — Schulungen ──
		c("DG.16", "Schulungen, Übungen und Sensibilisierung (§13 Abs.3 Nr.6)", "Regelmäßige Schulungen und Übungen für alle relevanten Mitarbeitenden (mind. jährlich). Nachweis: Schulungsplan, Teilnehmerlisten, Übungsberichte.", "Schulungen", "manual", 2),

		// ── Resilienzplan (§13 Abs.4) ──
		c("DG.18", "Resilienzplan (§13 Abs.4)", "Schriftlicher Resilienzplan dokumentiert alle Maßnahmen, Zuständigkeiten und Aktualisierungshistorie. Nachweis: Resilienzplan-Dokument (aktuell, unterschrieben), Versionierung.", "Resilienzplan", "document", 3),

		// ── Nachweispflichten (§16) ──
		c("DG.19", "Einreichung von Nachweisen (§16 Abs.1/3)", "Betreiber reichen alle 4 Jahre Nachweise über implementierte Maßnahmen beim BBK/BSI ein. Nachweis: Eingereichter Nachweisbericht, Bestätigung der Behörde.", "Nachweispflichten", "document", 3),
		c("DG.20", "Zusätzliche Nachweise und Resilienzplan (§16 Abs.2)", "Auf Anforderung werden zusätzliche Nachweise und der Resilienzplan an die Behörde übermittelt. Nachweis: Übermittlungsprotokolle, Behördenanfragen.", "Nachweispflichten", "document", 2),
		c("DG.21", "Behördliche Audits (§16 Abs.4)", "Betreiber ermöglichen behördliche Vor-Ort-Prüfungen. Nachweis: Audit-Unterstützungsprotokoll, Prüfbericht.", "Nachweispflichten", "manual", 2),
		c("DG.22", "Mängelbeseitigungsplan (§16 Abs.5)", "Bei festgestellten Mängeln wird ein Mängelbeseitigungsplan erstellt und eingereicht. Nachweis: Mängelbeseitigungsplan, Umsetzungsnachweise.", "Nachweispflichten", "manual", 2),

		// ── Meldepflichten (§18) ──
		c("DG.23", "Vorfallsmeldepflicht 24 Stunden (§18 Abs.1)", "Erhebliche Störungen der kritischen Anlage werden innerhalb von 24 Stunden beim BBK/BSI gemeldet. Nachweis: Meldeprotokoll, Eingangsbestätigungen, Meldeverfahrensdokumentation.", "Meldepflichten", "manual", 3),
		c("DG.24", "Informationen zu Vorfallsmeldungen (§18 Abs.2)", "Vorfallsmeldungen enthalten alle vorgeschriebenen Informationen. Nachweis: Meldevorlagen, ausgefüllte Meldungen.", "Meldepflichten", "document", 2),
		c("DG.25", "Öffentliche Informationspflicht (§18 Abs.9)", "Bei Vorfällen mit öffentlicher Relevanz werden Bevölkerung/Betroffene informiert. Nachweis: Kommunikationsplan, Veröffentlichungen.", "Meldepflichten", "manual", 2),

		// ── Leitungsverantwortung (§20) ──
		c("DG.26", "Leitungsverantwortung (§20 Abs.1)", "Das Leitungsorgan trägt die Gesamtverantwortung für die Umsetzung der Resilienzmaßnahmen und genehmigt den Resilienzplan. Nachweis: Unterschriebener Resilienzplan, Beschlussprotokoll.", "Leitungsverantwortung", "document", 3),
	}
}

// iso27017Controls returns controls for ISO/IEC 27017:2015 — Cloud Security.
// This standard provides guidelines for information security controls applicable
// to cloud service providers AND cloud service customers.
func iso27017Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// ── Gemeinsame Verantwortlichkeiten (CSP + CSC) ──────────────────
		c("27017-6.3.1", "Gemeinsame Rollen und Verantwortlichkeiten",
			"Dokumentiere die geteilten Verantwortlichkeiten zwischen Cloud-Anbieter (CSP) und Cloud-Nutzer (CSC) in einem Shared-Responsibility-Model. Definiere für jede Sicherheitsfunktion, wer zuständig ist. Nachweis: Shared-Responsibility-Matrix, Vertragsanhang.",
			"Governance", "document", 3),
		c("27017-6.3.2", "Entfernen und Rückgabe von Assets bei Vertragsende",
			"Stelle sicher, dass beim Vertragsende alle Kundendaten vollständig zurückgegeben oder nachweislich gelöscht werden. Definiere Exit-Prozeduren im Vertrag. Nachweis: Datenrückgabe-/Löschungsprotokoll, Vertragsklausel, Exit-Plan.",
			"Governance", "document", 3),
		// ── Asset Management ──────────────────────────────────────────────
		c("27017-8.1.1", "Inventarisierung von Cloud-Assets",
			"Führe ein vollständiges Inventar aller genutzten Cloud-Ressourcen (VMs, Buckets, Datenbanken, APIs). Automatisiere das Asset-Discovery wo möglich. Nachweis: Cloud-Asset-Register, Discovery-Tool-Report, Aktualisierungsfrequenz.",
			"Asset Management", "automated", 2),
		c("27017-8.1.3", "Handhabung von Cloud-Assets",
			"Definiere akzeptable Nutzungsregeln für Cloud-Ressourcen: Datenklassifizierung, Zugriffsrechte, Backup-Pflicht, Tagging-Standards. Nachweis: Cloud-Nutzungsrichtlinie, Tagging-Compliance-Report.",
			"Asset Management", "manual", 2),
		// ── Zugriffskontrolle ─────────────────────────────────────────────
		c("27017-9.1.2", "Zugriff auf Cloud-Dienste und -Ressourcen",
			"Implementiere Least-Privilege-Zugriff auf alle Cloud-Ressourcen: IAM-Rollen, Service Accounts, MFA für privilegierte Zugriffe, regelmäßige Access Reviews. Nachweis: IAM-Konfiguration, Access-Review-Protokoll, MFA-Aktivierungsnachweis.",
			"Zugriffskontrolle", "automated", 3),
		c("27017-9.4.4", "Schutz von privilegierten Utility-Programmen",
			"Kontrolliere und protokolliere den Zugriff auf Cloud-Management-APIs und Admin-Konsolen. Nutze separate privilegierte Konten, nie mit persönlicher Identität. Nachweis: Admin-Zugriffsprotokoll, PAM-Konfiguration.",
			"Zugriffskontrolle", "automated", 3),
		// ── Kryptographie ─────────────────────────────────────────────────
		c("27017-10.1.1", "Verschlüsselung in der Cloud",
			"Verschlüssele alle Daten at-rest und in-transit in Cloud-Umgebungen: AES-256 für ruhende Daten, TLS 1.2+ für Übertragungen, Customer-Managed Keys (CMK) für sensitive Daten. Nachweis: Verschlüsselungskonfiguration, KMS-Einstellungen, Zertifikatsstatus.",
			"Kryptographie", "automated", 3),
		c("27017-10.1.2", "Schlüsselverwaltung in der Cloud",
			"Verwalte kryptografische Schlüssel für Cloud-Dienste: Schlüsselrotation (jährlich), Hardware Security Modules (HSM) für kritische Schlüssel, Key-Escrow-Richtlinie. Nachweis: KMS-Konfiguration, Rotationsprotokoll, HSM-Nutzungsnachweis.",
			"Kryptographie", "automated", 3),
		// ── Physische und Umgebungssicherheit ─────────────────────────────
		c("27017-11.2.7", "Entsorgung von Cloud-Speichermedien",
			"Stelle sicher, dass beim Ableben/Austausch von Storage beim CSP Daten nachweislich gelöscht werden. Fordere Löschzertifikate. Nachweis: Löschzertifikat des CSP, Vertragsklausel zur sicheren Entsorgung.",
			"Physische Sicherheit", "document", 2),
		// ── Betriebssicherheit ────────────────────────────────────────────
		c("27017-12.1.3", "Kapazitätsmanagement in der Cloud",
			"Überwache und manage Cloud-Ressourcennutzung: automatisches Scaling, Budget-Alerts, Capacity-Reservierungen für kritische Workloads. Nachweis: Monitoring-Dashboard, Scaling-Konfiguration, Budget-Alert-Setup.",
			"Betriebssicherheit", "automated", 2),
		c("27017-12.4.1", "Ereignisprotokollierung in der Cloud",
			"Aktiviere umfassendes Logging für alle Cloud-Dienste (CloudTrail/Audit Logs): API-Aufrufe, Konfigurationsänderungen, Datenzugriffe. Zentralisiere Logs in unveränderlichem Storage. Nachweis: Logging-Konfiguration, Log-Archiv, Integritätsprüfung.",
			"Betriebssicherheit", "automated", 3),
		c("27017-12.6.1", "Schwachstellenmanagement für Cloud-Dienste",
			"Überwache Cloud-spezifische Schwachstellen: CSP-Security-Bulletins, Fehlkonfigurationen (CSPM-Tool), Container-Image-Schwachstellen. Definiere SLAs für die Behebung. Nachweis: CSPM-Scan-Ergebnisse, Patch-Protokoll, SLA-Dokumentation.",
			"Betriebssicherheit", "automated", 3),
		// ── Kommunikationssicherheit ──────────────────────────────────────
		c("27017-13.1.3", "Netzwerksegmentierung in der Cloud",
			"Implementiere Netzwerksegmentierung in Cloud-Umgebungen: Virtual Private Clouds (VPC), Security Groups, Network ACLs, Private Endpoints für kritische Dienste. Nachweis: Netzwerkarchitektur-Diagramm, VPC-Konfiguration, Flow-Log-Analyse.",
			"Kommunikationssicherheit", "automated", 3),
		// ── Lieferantenbeziehungen ────────────────────────────────────────
		c("27017-15.1.1", "Informationssicherheitsrichtlinie für Lieferanten (CSP)",
			"Prüfe und dokumentiere die Sicherheitszertifizierungen des Cloud-Anbieters (ISO 27001, SOC 2, C5-Attestierung, BSI-C5). Stelle sicher, dass Sicherheitsanforderungen im Vertrag verankert sind. Nachweis: CSP-Zertifikate, Vertragsanhang, jährliche Überprüfung.",
			"Lieferantenmanagement", "document", 3),
		c("27017-15.2.1", "Überwachung und Review von Cloud-Anbietern",
			"Überwache kontinuierlich die Sicherheitsperformance des Cloud-Anbieters: Status-Dashboard des CSP, Incident-Kommunikation, jährliche Security-Review. Nachweis: CSP-Monitoring-Dashboard, Incident-Kommunikationsprotokoll, Review-Bericht.",
			"Lieferantenmanagement", "manual", 2),
		// ── Cloud-spezifische Controls (CLD-Klausel) ──────────────────────
		c("27017-CLD.6.3.1", "Gemeinsame Sicherheitsmaßnahmen CSP und CSC",
			"Definiere explizit, welche Sicherheitsmaßnahmen der CSP implementiert und welche der CSC selbst implementieren muss. Basis: CSP-Sicherheitsweißbuch. Nachweis: Sicherheitsverantwortlichkeits-Matrix, bestätigtes CSP-Whitepaper.",
			"Cloud-Governance", "document", 3),
		c("27017-CLD.9.5.1", "Segregation in virtuellen Umgebungen",
			"Stelle sicher, dass Daten und Workloads verschiedener Mandanten (Tenants) strikt getrennt sind. Nutze dedizierte Ressourcen für hochsensitive Daten. Nachweis: Mandantentrennung-Dokumentation, Isolationstest-Bericht.",
			"Cloud-Governance", "automated", 3),
		c("27017-CLD.9.5.2", "VM-Härtung und sichere Administration",
			"Härte virtuelle Maschinen nach CIS-Benchmarks: keine Root-Logins via SSH, SSH-Key-Only-Authentifizierung, automatische Sicherheitsupdates, Deaktivierung nicht benötigter Dienste. Nachweis: CIS-Benchmark-Scan, VM-Konfigurationsnachweis.",
			"Cloud-Governance", "automated", 2),
		c("27017-CLD.12.4.5", "Monitoring und Alerting für Cloud-Dienste",
			"Implementiere umfassendes Cloud-Monitoring: Resource-Health, Security-Events, Kostenanomalien, Performance-Schwellwerte mit automatischer Alarmierung. Nachweis: Monitoring-Konfiguration, Alert-Regeln, On-Call-Prozess.",
			"Cloud-Governance", "automated", 2),
		// ── Ergänzende Cloud-Controls (ISO 27017 Annex A) ────────────────────
		c("27017-CLD.8.1.3", "Handhabung von Cloud-Assets und Speichermedien",
			"Definiere Verfahren für den Umgang mit Cloud-Speichermedien und virtuellen Datenträgern: Klassifizierung, Backup, Migration und sichere Löschung. Nachweis: Cloud-Storage-Richtlinie, Datenklassifizierungskonzept.",
			"Asset Management", "manual", 2),
		c("27017-6.1.1", "Informationssicherheitsrollen in der Cloud-Organisation",
			"Weise dedizierte Rollen für Cloud-Sicherheit zu (Cloud Security Engineer, Cloud Compliance Owner). Definiere Eskalationspfade für Cloud-Sicherheitsvorfälle. Nachweis: Organigramm, Stellenbeschreibungen, RACI-Matrix.",
			"Governance", "document", 2),
		c("27017-9.2.1", "Benutzerregistrierung und -abmeldung in Cloud-Diensten",
			"Führe einen formalisierten Prozess für die Zuweisung und den Entzug von Zugängen zu Cloud-Diensten. Automatisiere Provisioning/Deprovisioning via IaC oder IAM-Workflows. Nachweis: Onboarding/Offboarding-Checkliste, IAM-Auditlog.",
			"Zugriffskontrolle", "automated", 3),
		c("27017-9.2.3", "Verwaltung privilegierter Zugriffsrechte in Cloud-Umgebungen",
			"Verwalte privilegierte Cloud-Accounts (Cloud Root, Break-Glass-Accounts) nach dem Least-Privilege-Prinzip. Nutze JIT-Zugriff (Just-in-Time) und dokumentiere alle privilegierten Aktionen. Nachweis: PAM-Konfiguration, Break-Glass-Protokoll, JIT-Zugriffsregeln.",
			"Zugriffskontrolle", "automated", 3),
		c("27017-9.3.1", "Verwendung von geheimen Authentifizierungsinformationen",
			"Verwalte Cloud-Credentials (API-Keys, Secrets, Zertifikate) zentral via Secrets-Manager. Rotiere Secrets automatisch, verhindere Hardcoding in Code-Repositories. Nachweis: Secrets-Manager-Konfiguration, Git-Scan-Ergebnisse, Rotationsprotokoll.",
			"Zugriffskontrolle", "automated", 3),
		c("27017-12.3.1", "Backup-Strategie für Cloud-Daten",
			"Definiere und implementiere eine Cloud-spezifische Backup-Strategie (3-2-1-Cloud-Regel: 3 Kopien, 2 Standorte, 1 außerhalb des primären Cloud-Anbieters). Nachweis: Backup-Konfiguration, Cross-Region-Replikationsnachweis, Test-Restore-Protokoll.",
			"Betriebssicherheit", "automated", 3),
		c("27017-12.5.1", "Kontrolle von Software-Installationen in Cloud-Umgebungen",
			"Kontrolliere und genehmige alle Software-Installationen in Cloud-Instanzen: Whitelist-basierter Ansatz, Signatürprüfung, Infrastructure-as-Code für reproduzierbare Deployments. Nachweis: Deployment-Pipeline-Konfiguration, Image-Signaturrichtlinie.",
			"Betriebssicherheit", "automated", 2),
		c("27017-14.1.1", "Sicherheitsanforderungen in Cloud-Projekten",
			"Integriere Cloud-Security-Anforderungen in alle Projektphasen (Security by Design): Threat Modeling für Cloud-Architekturen, Security Reviews bei Infrastrukturänderungen. Nachweis: Cloud-Security-Checkliste, Architektur-Review-Protokolle.",
			"Systementwicklung", "manual", 2),
		c("27017-14.2.5", "Sichere Entwicklungsprinzipien für Cloud-Anwendungen",
			"Wende sichere Entwicklungsprinzipien für Cloud-native Anwendungen an: Container-Hardening, Secrets-Management in CI/CD, IaC-Security-Scanning (Checkov, tfsec). Nachweis: Security-Scan-Ergebnisse, Coding-Guidelines, Pipeline-Konfiguration.",
			"Systementwicklung", "automated", 2),
		c("27017-13.2.1", "Richtlinie und Verfahren für Informationsübertragung",
			"Definiere Richtlinien für sichere Dateiübertragungen zwischen Cloud-Umgebungen und On-Premise-Systemen (SFTP, HTTPS, VPN). Verbiete unverschlüsselte Protokolle. Nachweis: Übertragungsrichtlinie, DLP-Konfiguration, Proxy-Einstellungen.",
			"Kommunikationssicherheit", "manual", 2),
		c("27017-16.1.1", "Verantwortlichkeiten und Verfahren bei Cloud-Sicherheitsvorfällen",
			"Definiere Cloud-spezifische Incident-Response-Verfahren: Kontaktaufnahme mit CSP-Support, CSPM-Alert-Eskalation, Forensik in Cloud-Umgebungen (Read-Only-Snapshots). Nachweis: Cloud-Incident-Response-Plan, CSP-Support-Kontakte, Eskalationsmatrix.",
			"Incident Management", "manual", 3),
		c("27017-18.1.3", "Schutz von Aufzeichnungen in Cloud-Umgebungen",
			"Stelle sicher, dass Compliance-relevante Logs und Aufzeichnungen in der Cloud unveränderlich gespeichert werden (WORM-Storage, Log-Integrität via Hashing). Nachweis: Unveränderlichkeits-Konfiguration, Log-Integritätsprüfung, Retentionsrichtlinie.",
			"Compliance", "automated", 2),
	}
}

// iso27018Controls returns controls for ISO/IEC 27018:2019 — Cloud Privacy.
// Code of practice for protection of personally identifiable information (PII)
// in public clouds acting as PII processors (Art. 28 DSGVO processors).
func iso27018Controls(frameworkID, orgID string) []Control {
	c := func(id, title, desc, domain, evType string, w int) Control {
		return Control{FrameworkID: frameworkID, OrgID: orgID, ControlID: id, Title: title, Description: desc, Domain: domain, EvidenceType: evType, Weight: w}
	}
	return []Control{
		// ── Einwilligung und Zweckbindung ─────────────────────────────────
		c("27018-A.1.1", "Zweckbindung bei PII-Verarbeitung",
			"Verarbeite personenbezogene Daten (PII) in der Cloud ausschließlich für dokumentierte, vertraglich vereinbarte Zwecke. Keine Verarbeitung für eigene Geschäftszwecke des CSP ohne ausdrückliche Genehmigung. Nachweis: AVV-Vertrag, Verarbeitungszweck-Dokumentation.",
			"Datenschutz-Governance", "document", 3),
		c("27018-A.1.2", "Zustimmungsmanagement",
			"Implementiere ein Consent-Management-System für alle PII-Verarbeitungsvorgänge, die Einwilligung erfordern. Dokumentiere Einwilligungen mit Zeitstempel und Widerrufsoptionen. Nachweis: CMP-Konfiguration, Einwilligungsprotokoll, Widerrufsprozess.",
			"Datenschutz-Governance", "automated", 3),
		// ── Rechtliche Grundlagen ──────────────────────────────────────────
		c("27018-A.2.1", "Rechtliche Grundlage für PII-Übertragungen",
			"Stelle sicher, dass alle Übertragungen von PII in die Cloud eine rechtliche Grundlage haben (Art. 6 DSGVO). Für Drittlandübertragungen: Angemessenheitsbeschluss oder SCCs dokumentieren. Nachweis: Datenschutz-Rechtsgutachten, SCC-Dokumentation, EU-US Data Privacy Framework.",
			"Rechtliche Grundlagen", "document", 3),
		// ── Betroffenenrechte ──────────────────────────────────────────────
		c("27018-A.3.1", "Unterstützung bei Betroffenenrechten",
			"Als Cloud-Auftragsverarbeiter: unterstütze den Verantwortlichen bei der Erfüllung von Auskunfts-, Berichtigungs-, Löschungs- und Widerspruchsrechten. Definiere SLAs für die Reaktion. Nachweis: Verfahrensbeschreibung, SLA-Dokumentation, Testprotokoll eines Löschvorgangs.",
			"Betroffenenrechte", "manual", 3),
		c("27018-A.3.2", "Löschung und Rückgabe von PII",
			"Lösche oder gib alle PII zurück, wenn der Verarbeitungsauftrag endet oder der Auftraggeber dies verlangt. Dokumentiere den Löschvorgang nachvollziehbar. Nachweis: Löschprotokoll, Bestätigungsschreiben, technischer Löschnachweis.",
			"Betroffenenrechte", "manual", 3),
		// ── Transparenz und Offenlegung ───────────────────────────────────
		c("27018-A.4.1", "Offenlegung von Sub-Auftragsverarbeitern",
			"Informiere den Auftraggeber über alle eingesetzten Sub-Auftragsverarbeiter (Unterauftragnehmer). Stelle sicher, dass Sub-AVVs dieselben Datenschutzanforderungen enthalten. Nachweis: Sub-AV-Liste, Sub-AVV-Verträge, Benachrichtigungsprozess bei Änderungen.",
			"Transparenz", "document", 3),
		c("27018-A.4.2", "Behördliche Zugriffsanfragen",
			"Definiere Verfahren für staatliche Zugriffsanfragen auf PII (§ 100g StPO, FISA): sofortige Benachrichtigung des Auftraggebers soweit rechtlich möglich, Anforderung eines Gerichtsbeschlusses, Dokumentation aller Anfragen. Nachweis: Verfahrensdokumentation, geschwärzte Anfragenübersicht.",
			"Transparenz", "document", 3),
		c("27018-A.4.3", "Standorte der Datenverarbeitung",
			"Dokumentiere alle geografischen Standorte, an denen PII gespeichert oder verarbeitet wird. Stelle Auftraggeber-Kontrolle über Datenresidenz sicher. Nachweis: Rechenzentrum-Standortliste, Vertragliche Datenresidenz-Klausel, Cloud-Region-Konfiguration.",
			"Transparenz", "document", 2),
		// ── Datensicherheit ───────────────────────────────────────────────
		c("27018-A.5.1", "Verschlüsselung von PII at-rest",
			"Verschlüssele alle PII in der Cloud mit starker Verschlüsselung (AES-256). Nutze Customer-Managed Encryption Keys (CMEK) für maximale Kontrolle. Nachweis: Verschlüsselungskonfiguration, CMEK-Nachweis, Schlüsselverwaltungsrichtlinie.",
			"Datensicherheit", "automated", 3),
		c("27018-A.5.2", "Verschlüsselung von PII in der Übertragung",
			"Verschlüssele alle PII-Übertragungen mit TLS 1.2 oder höher. Verbiete unverschlüsselte Protokolle (HTTP, FTP). Nachweis: TLS-Konfigurationsnachweis, SSL-Labs-Scan, Netzwerk-Policy.",
			"Datensicherheit", "automated", 3),
		c("27018-A.5.3", "Zugriffskontrolle für PII",
			"Implementiere Need-to-Know-Zugriffskontrolle für PII: rollenbasierter Zugriff, Protokollierung aller PII-Zugriffe, regelmäßige Access Reviews. Nachweis: IAM-Konfiguration, Zugriffsprotokoll, Access-Review-Ergebnis.",
			"Datensicherheit", "automated", 3),
		// ── Vorfallsmanagement ────────────────────────────────────────────
		c("27018-A.6.1", "Meldung von PII-Sicherheitsvorfällen",
			"Melde PII-Sicherheitsvorfälle (Data Breaches) unverzüglich an den Auftraggeber (gem. Art. 33 DSGVO: innerhalb 72h). Dokumentiere Vorfall, Auswirkungen und ergriffene Maßnahmen. Nachweis: Incident-Response-Plan, Meldeprozess-Dokumentation, Meldeprotokoll.",
			"Vorfallsmanagement", "manual", 3),
		// ── Mitarbeiter und Zugriff ───────────────────────────────────────
		c("27018-A.7.1", "Vertraulichkeitsverpflichtung für PII-Zugriff",
			"Verpflichte alle Mitarbeiter mit PII-Zugriff auf Vertraulichkeit. Führe spezifische Datenschutzschulungen durch. Stelle Zugriff nur nach Need-to-Know bereit. Nachweis: Vertraulichkeitserklärungen, Schulungsnachweise, Zugriffsprotokoll.",
			"Personal", "manual", 2),
		c("27018-A.7.2", "Einschränkung von Kopierfunktionen",
			"Verhindere das unbefugte Kopieren und Übertragen von PII aus der Cloud-Umgebung: DLP-Systeme, eingeschränkte Export-Funktionen, USB-Blockierung an Admin-Systemen. Nachweis: DLP-Konfiguration, Exportprotokoll.",
			"Personal", "automated", 2),
		// ── Rechenschaftspflicht ──────────────────────────────────────────
		c("27018-A.8.1", "Aufzeichnung von PII-Zugriffen",
			"Führe vollständige Audit-Logs aller PII-Zugriffe: wer hat wann auf welche PII zugegriffen, geändert oder gelöscht. Schütze Logs vor Manipulation. Nachweis: Audit-Log-Konfiguration, Integritätsnachweis, Log-Aufbewahrungsrichtlinie.",
			"Rechenschaftspflicht", "automated", 3),
		c("27018-A.8.2", "Compliance-Prüfung",
			"Führe regelmäßige interne Audits der ISO-27018-Konformität durch. Lasse externe Audits und Zertifizierungen durch akkreditierte Stellen durchführen. Nachweis: Interner Audit-Bericht, externes Zertifikat (SOC 2 Typ II oder ISO 27018-Zertifikat).",
			"Rechenschaftspflicht", "document", 2),
	}
}

// EnrichControlsWithNIS2Meta sets RegulationSource, ThematicArea, and ApplicabilityScope
// on any control whose ID appears in nis2ControlMeta (S70-2).
func EnrichControlsWithNIS2Meta(cs []Control) {
	for i := range cs {
		if m, ok := nis2ControlMeta[cs[i].ControlID]; ok {
			cs[i].RegulationSource = m.source
			cs[i].ThematicArea = m.area
			cs[i].ApplicabilityScope = m.scopes
		}
	}
}
