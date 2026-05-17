package secvitals

import "gopkg.in/yaml.v3"

// yamlUnmarshal decodes YAML bytes into v.
func yamlUnmarshal(data []byte, v any) error {
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
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
	IsEnabled   bool   `json:"is_enabled"`
}

// builtinAvailable is the catalogue of all built-in frameworks.
var builtinAvailable = []struct {
	name        string
	description string
}{
	{"NIS2", "EU-Richtlinie zur Netz- und Informationssicherheit (NIS 2) — gilt für wesentliche und wichtige Einrichtungen."},
	{"ISO27001", "ISO/IEC 27001:2022 — Internationaler Standard für Informationssicherheits-Managementsysteme (ISMS)."},
	{"BSI", "BSI IT-Grundschutz — Deutschen Standard des Bundesamts für Sicherheit in der Informationstechnik."},
	{"CRA", "EU Cyber Resilience Act (CRA) — Sicherheitsanforderungen für Produkte mit digitalen Elementen."},
	{"DORA", "Digital Operational Resilience Act (DORA) — ICT-Resilienzanforderungen für Finanzunternehmen."},
	{"EUAIACT", "EU AI Act — Anforderungen für KI-Systeme, insbesondere Hochrisiko-KI (Anhang III)."},
	{"ISO42001", "ISO/IEC 42001:2023 — KI-Managementsystem-Standard für verantwortungsvolle KI-Entwicklung und -Nutzung."},
	{"TISAX", "TISAX® / VDA ISA — Informationssicherheitsstandard der Automobilindustrie für Zulieferer mit OEM-Datenzugang."},
	{"DSGVO-TOM", "DSGVO Art. 32 TOMs — Technische und organisatorische Maßnahmen gemäß Art. 32 DSGVO, abgeleitet aus ISO 27001."},
	{"CIS", "CIS Controls v8 — 18 Kontrollgruppen des Center for Internet Security. IG1-Safeguards als Mindestanforderung für alle Organisationen."},
}
