// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bsi

import "time"

// ── S61-5: BSI Baustein-Modellierung ──

// BSIModelingEntry represents a single Baustein-to-Asset mapping row.
type BSIModelingEntry struct {
	ID                        string    `json:"id"`
	OrgID                     string    `json:"org_id"`
	AssetID                   string    `json:"asset_id"`
	ControlID                 string    `json:"control_id"`
	Priority                  string    `json:"priority"`
	JustificationForExclusion string    `json:"justification_for_exclusion"`
	CheckStatus               *string   `json:"check_status,omitempty"`
	InterviewNotes            string    `json:"interview_notes"`
	SiteVisitNotes            string    `json:"site_visit_notes"`
	AssetName                 string    `json:"asset_name"`
	ControlTitle              string    `json:"control_title"`
	FrameworkID               string    `json:"framework_id"`
	CreatedBy                 string    `json:"created_by"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// CreateBSIModelingInput holds validated input for creating a BSI modeling entry.
type CreateBSIModelingInput struct {
	AssetID                   string  `json:"asset_id"   validate:"required"`
	ControlID                 string  `json:"control_id" validate:"required"`
	Priority                  string  `json:"priority"   validate:"required,oneof=R1 R2 R3"`
	JustificationForExclusion string  `json:"justification_for_exclusion"`
	CheckStatus               *string `json:"check_status,omitempty" validate:"omitempty,oneof=yes partial no not_applicable"`
	InterviewNotes            string  `json:"interview_notes"`
	SiteVisitNotes            string  `json:"site_visit_notes"`
}

// UpdateBSIModelingInput holds validated input for updating a BSI modeling entry.
type UpdateBSIModelingInput struct {
	Priority                  string  `json:"priority"   validate:"required,oneof=R1 R2 R3"`
	JustificationForExclusion string  `json:"justification_for_exclusion"`
	CheckStatus               *string `json:"check_status,omitempty" validate:"omitempty,oneof=yes partial no not_applicable"`
	InterviewNotes            string  `json:"interview_notes"`
	SiteVisitNotes            string  `json:"site_visit_notes"`
}

// BSIModelingStats holds aggregate check-status counts for a BSI modeling matrix.
type BSIModelingStats struct {
	Total        int `json:"total"`
	CountYes     int `json:"count_yes"`
	CountPartial int `json:"count_partial"`
	CountNo      int `json:"count_no"`
	CountNA      int `json:"count_na"`
	CountPending int `json:"count_pending"`
}

// ── S74-1: IT-Grundschutz-Check-Workflow ──

// BSITargetObject represents a Zielobjekt in the IT-Grundschutz Strukturanalyse.
type BSITargetObject struct {
	ID                 string  `json:"id"`
	OrgID              string  `json:"org_id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	Description        string  `json:"description"`
	ProtectionC        *string `json:"protection_c,omitempty"`
	ProtectionI        *string `json:"protection_i,omitempty"`
	ProtectionA        *string `json:"protection_a,omitempty"`
	Absicherungsniveau string  `json:"absicherungsniveau"`
	// Override fields (optional — set via PUT .../protection-override)
	OverrideC      *string `json:"override_c,omitempty"`
	OverrideI      *string `json:"override_i,omitempty"`
	OverrideA      *string `json:"override_a,omitempty"`
	OverrideReason *string `json:"override_reason,omitempty"`
	OverrideEffect *string `json:"override_effect,omitempty"`
	// Effective values computed on-read (Maximumprinzip + override, ADR-0054)
	EffectiveC     *string   `json:"effective_c,omitempty"`
	EffectiveI     *string   `json:"effective_i,omitempty"`
	EffectiveA     *string   `json:"effective_a,omitempty"`
	InheritedFromC *string   `json:"inherited_from_c,omitempty"`
	InheritedFromI *string   `json:"inherited_from_i,omitempty"`
	InheritedFromA *string   `json:"inherited_from_a,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BSIObjectDependency represents a Abhängigkeitskante zwischen zwei Zielobjekten.
type BSIObjectDependency struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	SourceID       string    `json:"source_id"`
	SourceName     string    `json:"source_name"`
	TargetID       string    `json:"target_id"`
	TargetName     string    `json:"target_name"`
	DependencyType string    `json:"dependency_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateBSIObjectDependencyInput holds validated input for adding a dependency edge.
type CreateBSIObjectDependencyInput struct {
	TargetID       string `json:"target_id"        validate:"required,uuid"`
	DependencyType string `json:"dependency_type"  validate:"required,oneof=runs_on located_in connected_to processes_for"`
}

// UpdateBSIObjectProtectionOverrideInput holds validated input for CIA override.
type UpdateBSIObjectProtectionOverrideInput struct {
	OverrideC      *string `json:"override_c,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	OverrideI      *string `json:"override_i,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	OverrideA      *string `json:"override_a,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	OverrideReason string  `json:"override_reason"       validate:"max=500"`
	// kumulation = Override increases value; verteilung = Override decreases value
	OverrideEffect *string `json:"override_effect,omitempty" validate:"omitempty,oneof=kumulation verteilung"`
}

// CreateBSITargetObjectInput holds validated input for creating a Zielobjekt.
type CreateBSITargetObjectInput struct {
	Name               string  `json:"name"                validate:"required,max=200"`
	Type               string  `json:"type"                validate:"required,oneof=it_system application network room process"`
	Description        string  `json:"description"`
	ProtectionC        *string `json:"protection_c,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	ProtectionI        *string `json:"protection_i,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	ProtectionA        *string `json:"protection_a,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	Absicherungsniveau string  `json:"absicherungsniveau"  validate:"omitempty,oneof=basis standard kern"`
}

// UpdateBSITargetObjectInput holds validated input for updating a Zielobjekt.
type UpdateBSITargetObjectInput struct {
	Name               string  `json:"name"                validate:"required,max=200"`
	Type               string  `json:"type"                validate:"required,oneof=it_system application network room process"`
	Description        string  `json:"description"`
	ProtectionC        *string `json:"protection_c,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	ProtectionI        *string `json:"protection_i,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	ProtectionA        *string `json:"protection_a,omitempty"  validate:"omitempty,oneof=normal hoch sehr_hoch"`
	Absicherungsniveau string  `json:"absicherungsniveau"  validate:"omitempty,oneof=basis standard kern"`
}

// BSICheckResult represents one Anforderung × Zielobjekt status in the IT-Grundschutz-Check.
type BSICheckResult struct {
	ID               string `json:"id"`
	OrgID            string `json:"org_id"`
	TargetObjectID   string `json:"target_object_id"`
	BausteinID       string `json:"baustein_id"`
	AnforderungID    string `json:"anforderung_id"`
	AnforderungTitle string `json:"anforderung_title,omitempty"`
	// RequirementLevel is "basis", "standard", or "erhoeht" (from ck_controls.requirement_level).
	RequirementLevel string    `json:"requirement_level,omitempty"`
	Umsetzungsstatus string    `json:"umsetzungsstatus"`
	Begruendung      string    `json:"begruendung"`
	Verantwortlicher string    `json:"verantwortlicher"`
	Umsetzungsdatum  *string   `json:"umsetzungsdatum,omitempty"`
	Notiz            string    `json:"notiz"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// SetCheckResultInput holds validated input for setting a check result.
type SetCheckResultInput struct {
	Umsetzungsstatus string  `json:"umsetzungsstatus" validate:"required,oneof=entbehrlich ja teilweise nein"`
	Begruendung      string  `json:"begruendung"`
	Verantwortlicher string  `json:"verantwortlicher"`
	Umsetzungsdatum  *string `json:"umsetzungsdatum,omitempty"`
	Notiz            string  `json:"notiz"`
}

// BulkCheckResultItem is one item in a bulk update request.
type BulkCheckResultItem struct {
	AnforderungID    string  `json:"anforderung_id"    validate:"required"`
	Umsetzungsstatus string  `json:"umsetzungsstatus"  validate:"required,oneof=entbehrlich ja teilweise nein"`
	Begruendung      string  `json:"begruendung"`
	Verantwortlicher string  `json:"verantwortlicher"`
	Umsetzungsdatum  *string `json:"umsetzungsdatum,omitempty"`
	Notiz            string  `json:"notiz"`
}

// CheckSummary holds aggregated progress for one Zielobjekt.
type CheckSummary struct {
	TargetObjectID     string  `json:"target_object_id"`
	TotalAnforderungen int     `json:"total_anforderungen"`
	CountJa            int     `json:"count_ja"`
	CountTeilweise     int     `json:"count_teilweise"`
	CountNein          int     `json:"count_nein"`
	CountEntbehrlich   int     `json:"count_entbehrlich"`
	UmsetzungsgradPct  float64 `json:"umsetzungsgrad_pct"`
}

// AssignBausteinInput holds input for assigning a Baustein to a Zielobjekt.
type AssignBausteinInput struct {
	BausteinID string `json:"baustein_id" validate:"required"`
}

// ── S74-2: Grundschutz-Cockpit & GAP-Report ──

// BSICockpit holds dashboard data: heatmap, top gaps, overall progress.
type BSICockpit struct {
	GesamtFortschrittPct float64       `json:"gesamt_fortschritt_pct"`
	Heatmap              []HeatmapRow  `json:"heatmap"`
	TopGaps              []BSIGapEntry `json:"top_gaps"`
	UeberfaelligCount    int           `json:"ueberfaellig_count"`
}

// HeatmapRow represents one Baustein row in the cockpit heatmap.
type HeatmapRow struct {
	BausteinID    string        `json:"baustein_id"`
	BausteinTitle string        `json:"baustein_title"`
	Cells         []HeatmapCell `json:"cells"`
}

// HeatmapCell is one Baustein × Zielobjekt cell in the heatmap.
type HeatmapCell struct {
	TargetObjectID   string  `json:"target_object_id"`
	TargetObjectName string  `json:"target_object_name"`
	FortschrittPct   float64 `json:"fortschritt_pct"`
}

// BSIGapEntry represents one open requirement across multiple Zielobjekte.
type BSIGapEntry struct {
	BausteinID            string   `json:"baustein_id"`
	AnforderungID         string   `json:"anforderung_id"`
	AnforderungTitle      string   `json:"anforderung_title"`
	BetroffeneZielobjekte []string `json:"betroffene_zielobjekte"`
	Status                string   `json:"status"`
}

// BSIGapReport holds the full GAP report data.
type BSIGapReport struct {
	OrgID               string         `json:"org_id"`
	GeneratedAt         time.Time      `json:"generated_at"`
	GesamtAnforderungen int            `json:"gesamt_anforderungen"`
	GesamtEntbehrlich   int            `json:"gesamt_entbehrlich"`
	GesamtJa            int            `json:"gesamt_ja"`
	GesamtTeilweise     int            `json:"gesamt_teilweise"`
	GesamtNein          int            `json:"gesamt_nein"`
	UmsetzungsgradPct   float64        `json:"umsetzungsgrad_pct"`
	Gaps                []BSIGapDetail `json:"gaps"`
}

// BSIGapDetail is one entry in the full GAP report.
type BSIGapDetail struct {
	BausteinID       string `json:"baustein_id"`
	AnforderungID    string `json:"anforderung_id"`
	AnforderungTitle string `json:"anforderung_title"`
	Zielobjekt       string `json:"zielobjekt"`
	Umsetzungsstatus string `json:"umsetzungsstatus"`
	Verantwortlicher string `json:"verantwortlicher"`
	Umsetzungsdatum  string `json:"umsetzungsdatum,omitempty"`
}

// ── S74-3: Risikobewertung BSI 200-3 ──

// BSIThreat represents one of the 47 elementare Gefährdungen (G-0.x).
type BSIThreat struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// BSIRiskAssessment represents one risk entry in a BSI 200-3 risk analysis.
type BSIRiskAssessment struct {
	ID                   string    `json:"id"`
	OrgID                string    `json:"org_id"`
	TargetObjectID       string    `json:"target_object_id"`
	ThreatID             string    `json:"threat_id"`
	ThreatTitle          string    `json:"threat_title,omitempty"`
	Eintrittshaeufigkeit string    `json:"eintrittshaeufigkeit"`
	Schadensauswirkung   string    `json:"schadensauswirkung"`
	Risikokategorie      string    `json:"risikokategorie"`
	Behandlungsoption    *string   `json:"behandlungsoption,omitempty"`
	Massnahme            string    `json:"massnahme"`
	Verantwortlicher     string    `json:"verantwortlicher"`
	Zieldatum            *string   `json:"zieldatum,omitempty"`
	Restrisiko           *string   `json:"restrisiko,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CreateBSIRiskInput holds validated input for adding a risk entry.
type CreateBSIRiskInput struct {
	ThreatID             string `json:"threat_id"             validate:"required"`
	Eintrittshaeufigkeit string `json:"eintrittshaeufigkeit"  validate:"required,oneof=selten mittel haeufig sehr_haeufig"`
	Schadensauswirkung   string `json:"schadensauswirkung"    validate:"required,oneof=vernachlaessigbar begrenzt betraechtlich existenzbedrohend"`
}

// UpdateBSIRiskInput holds validated input for updating a risk entry.
type UpdateBSIRiskInput struct {
	Eintrittshaeufigkeit string  `json:"eintrittshaeufigkeit"  validate:"required,oneof=selten mittel haeufig sehr_haeufig"`
	Schadensauswirkung   string  `json:"schadensauswirkung"    validate:"required,oneof=vernachlaessigbar begrenzt betraechtlich existenzbedrohend"`
	Behandlungsoption    *string `json:"behandlungsoption,omitempty" validate:"omitempty,oneof=reduzieren akzeptieren vermeiden transferieren"`
	Massnahme            string  `json:"massnahme"`
	Verantwortlicher     string  `json:"verantwortlicher"`
	Zieldatum            *string `json:"zieldatum,omitempty"`
	Restrisiko           *string `json:"restrisiko,omitempty" validate:"omitempty,oneof=gering mittel hoch sehr_hoch"`
}

// BSIRiskSummary holds aggregated risk counts by category.
type BSIRiskSummary struct {
	Gering   int `json:"gering"`
	Mittel   int `json:"mittel"`
	Hoch     int `json:"hoch"`
	SehrHoch int `json:"sehr_hoch"`
	Offen    int `json:"offen"`
}

// ── S74-4: Referenzberichte A1–A6 ──

// BSIReportExport represents one entry in the report audit log.
type BSIReportExport struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	ReportType    string    `json:"report_type"`
	GeneratedBy   *string   `json:"generated_by,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
	SHA256        string    `json:"sha256"`
	FileSizeBytes *int      `json:"file_size_bytes,omitempty"`
	Metadata      any       `json:"metadata"`
}
