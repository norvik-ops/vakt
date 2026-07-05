// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import "gopkg.in/yaml.v3"

// yamlUnmarshal decodes YAML bytes into v.
func YAMLUnmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

// FrameworkPlugin defines the YAML schema for installable compliance framework plugins.
//
// Example plugin file:
//
//	name: "MyFramework"
//	version: "2024"
//	description: "Custom compliance framework"
//	controls:
//	  - id: "MF-1.1"
//	    title: "Control Title"
//	    description: "Control description"
//	    domain: "Risikomanagement"
//	    evidence_type: "manual"
//	    weight: 2
type FrameworkPlugin struct {
	Name        string          `yaml:"name"`
	Version     string          `yaml:"version"`
	Description string          `yaml:"description"`
	Controls    []PluginControl `yaml:"controls"`
}

// PluginControl is a single control definition inside a framework plugin YAML.
type PluginControl struct {
	ID           string `yaml:"id"`
	Title        string `yaml:"title"`
	Description  string `yaml:"description"`
	Domain       string `yaml:"domain"`
	EvidenceType string `yaml:"evidence_type"` // manual, automated, third_party
	Weight       int    `yaml:"weight"`
}

// AvailableFramework describes a framework available for installation.
type AvailableFramework struct {
	Name                string `json:"name"`
	Version             string `json:"version"`
	Description         string `json:"description"`
	IsBuiltin           bool   `json:"is_builtin"`
	IsEnabled           bool   `json:"is_enabled"`
	Status              string `json:"status,omitempty"`               // "draft" or "" (stable)
	ExpectedPublication string `json:"expected_publication,omitempty"` // "2026-12-31" or ""
}

// builtinAvailable is the catalogue of all built-in frameworks.
var builtinAvailable = []struct {
	name                string
	description         string
	status              string // "draft" for pre-publication standards; empty for stable
	expectedPublication string // expected publication date for draft standards; empty for stable
}{
	{"NIS2", "EU-Richtlinie zur Netz- und Informationssicherheit (NIS 2) — gilt für wesentliche und wichtige Einrichtungen.", "", ""},
	{"ISO27001", "ISO/IEC 27001:2022 — Internationaler Standard für Informationssicherheits-Managementsysteme (ISMS).", "", ""},
	{"BSI", "BSI IT-Grundschutz — Deutschen Standard des Bundesamts für Sicherheit in der Informationstechnik.", "", ""},
	{"CRA", "EU Cyber Resilience Act (CRA) — Sicherheitsanforderungen für Produkte mit digitalen Elementen.", "", ""},
	{"DORA", "Digital Operational Resilience Act (DORA) — ICT-Resilienzanforderungen für Finanzunternehmen.", "draft", ""},
	{"EUAIACT", "EU AI Act — Anforderungen für KI-Systeme, insbesondere Hochrisiko-KI (Anhang III).", "", ""},
	{"ISO42001", "ISO/IEC 42001:2023 — KI-Managementsystem-Standard für verantwortungsvolle KI-Entwicklung und -Nutzung.", "", ""},
	{"TISAX", "TISAX® / VDA ISA — Informationssicherheitsstandard der Automobilindustrie für Zulieferer mit OEM-Datenzugang.", "draft", ""},
	{"DSGVO-TOM", "DSGVO Art. 32 TOMs — Technische und organisatorische Maßnahmen gemäß Art. 32 DSGVO, abgeleitet aus ISO 27001.", "", ""},
	{"CIS", "CIS Controls v8 — 18 Kontrollgruppen des Center for Internet Security. IG1-Safeguards als Mindestanforderung für alle Organisationen.", "", ""},
	{"KRITIS", "KRITIS-DachG — Resilienzanforderungen für Betreiber kritischer Anlagen in Deutschland (§§ 8, 12, 13, 16, 18, 20 KRITIS-DachG, BGBl. 2026 I Nr. 66).", "", ""},
	{"C5", "BSI C5:2020 — Cloud Computing Compliance Criteria Catalogue. Prüfgrundlage für Cloud-Dienste in Deutschland (Hetzner, IONOS, AWS-DE, Azure-DE).", "", ""},
	{"ISO27017", "ISO/IEC 27017:2015 — Code of Practice für Informationssicherheitsmaßnahmen für Cloud-Dienste (CSP & CSC). Ergänzt ISO 27001 um 7 Cloud-spezifische Controls.", "", ""},
	{"ISO27018", "ISO/IEC 27018:2019 — Code of Practice zum Schutz personenbezogener Daten (PII) in Public Clouds. Auftragsverarbeiter nach Art. 28 DSGVO.", "", ""},
	{"prEN18286", "prEN 18286 — EU AI Act harmonisierter Standard (Entwurf). KI-Managementsystem-Anforderungen komplementär zu ISO/IEC 42001:2023. Publikation erwartet Ende 2026.", "draft", "2026-12-31"},
}
