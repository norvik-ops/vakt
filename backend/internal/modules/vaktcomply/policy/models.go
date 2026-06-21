// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import "time"

// Framework represents a compliance framework enabled for an organisation.
type Framework struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	Name             string    `json:"name"`
	Version          string    `json:"version"`
	IsBuiltin        bool      `json:"is_builtin"`
	ReadinessScore   float64   `json:"readiness_score,omitempty"`
	FrameworkVariant string    `json:"framework_variant"`         // "full" | "simplified" (DORA Art.16)
	CatalogEdition   string    `json:"catalog_edition,omitempty"` // S82-4: edition from the embedded catalog
	CreatedAt        time.Time `json:"created_at"`
}

// EnableFrameworkInput holds optional body params for POST /frameworks/:name/enable.
type EnableFrameworkInput struct {
	Variant string `json:"variant" validate:"omitempty,oneof=full simplified"`
}

// SwitchDORAVariantInput holds validated input for PUT /frameworks/dora/variant.
type SwitchDORAVariantInput struct {
	Variant string `json:"variant" validate:"required,oneof=full simplified"`
}

// Control represents an individual compliance control within a framework.
type Control struct {
	ID                  string `json:"id"`
	FrameworkID         string `json:"framework_id"`
	OrgID               string `json:"org_id"`
	ControlID           string `json:"control_id"`
	Title               string `json:"title"`
	Description         string `json:"description,omitempty"`
	Domain              string `json:"domain"`
	EvidenceType        string `json:"evidence_type"`
	Weight              int    `json:"weight"`
	EvidenceCount       int    `json:"evidence_count,omitempty"`
	Status              string `json:"status,omitempty"` // computed: covered/partial/missing/not_applicable/in_progress/implemented
	NotApplicable       bool   `json:"not_applicable"`
	NotApplicableReason string `json:"not_applicable_reason,omitempty"`
	ManualStatus        string `json:"manual_status,omitempty"` // "" | "in_progress" | "implemented"
	ISO27001Mapping     string `json:"iso27001_mapping,omitempty"`
	MaturityScore       int    `json:"maturity_score"` // 0–3 (TISAX VDA ISA maturity level)
	Owner               string `json:"owner,omitempty"`
	// Review tracking (Migration 075)
	LastReviewedAt     *time.Time `json:"last_reviewed_at"`
	ReviewIntervalDays int        `json:"review_interval_days"`
	NextReviewDue      *time.Time `json:"next_review_due"`
	LastReviewedBy     string     `json:"last_reviewed_by"`
	ReviewNote         string     `json:"review_note"`
	IsReviewOverdue    bool       `json:"is_review_overdue"` // computed: next_review_due < NOW() AND next_review_due IS NOT NULL
	DueDate            *time.Time `json:"due_date,omitempty"`
	// Evidence staleness (Migration 178, S67-4)
	EvidenceStatus     string     `json:"evidence_status,omitempty"` // ok | stale | missing | na
	EvidenceMaxAgeDays *int       `json:"evidence_max_age_days,omitempty"`
	EvidenceExpiresAt  *time.Time `json:"evidence_expires_at,omitempty"`
	// NIS2 enrichment (S70-2)
	RegulationSource   string   `json:"regulation_source,omitempty"`
	ThematicArea       string   `json:"thematic_area,omitempty"`
	ApplicabilityScope []string `json:"applicability_scope,omitempty"`
}

// ControlReview represents a single periodic review event for a control.
type ControlReview struct {
	ID             string    `json:"id"`
	ControlID      string    `json:"control_id"`
	ReviewedBy     string    `json:"reviewed_by"`
	ReviewNote     string    `json:"review_note"`
	StatusAtReview string    `json:"status_at_review"`
	ReviewedAt     time.Time `json:"reviewed_at"`
}

// RecordReviewInput holds validated input for recording a control review.
type RecordReviewInput struct {
	ReviewedBy     string `json:"reviewed_by"          validate:"required,max=200"`
	ReviewNote     string `json:"review_note"          validate:"max=2000"`
	ReviewInterval int    `json:"review_interval_days" validate:"omitempty,min=30,max=3650"`
}

// UpdateControlInput holds input for PATCH /vaktcomply/controls/:id.
// not_applicable takes precedence over manual_status; set manual_status="" to reset to computed.
type UpdateControlInput struct {
	NotApplicable bool    `json:"not_applicable"`
	Reason        string  `json:"reason"`
	ManualStatus  string  `json:"manual_status" validate:"omitempty,oneof=in_progress implemented"`
	MaturityScore *int    `json:"maturity_score" validate:"omitempty,min=0,max=3"`
	Owner         string  `json:"owner"          validate:"omitempty,max=200"`
	DueDate       *string `json:"due_date"      validate:"omitempty,datetime=2006-01-02"`
}

// BulkUpdateControlsInput holds input for PATCH /vaktcomply/controls/bulk.
type BulkUpdateControlsInput struct {
	IDs    []string `json:"ids"    validate:"required,min=1,max=100"`
	Status string   `json:"status" validate:"required,oneof=implemented in_progress not_implemented not_applicable"`
}

// UpdateSoAMetadataInput holds SoA-specific fields for PATCH /vaktcomply/controls/:id/soa.
type UpdateSoAMetadataInput struct {
	Justification  string `json:"justification"`
	Implementation string `json:"implementation"`
	Responsible    string `json:"responsible"`
}

// SoARow is a flattened view of a control enriched with SoA metadata and evidence count,
// used exclusively by the SoA PDF generator.
type SoARow struct {
	ControlID      string
	Title          string
	Domain         string
	Applicable     bool
	Justification  string
	Implementation string
	Responsible    string
	ManualStatus   string
	EvidenceCount  int
}

// TISAXControlGap describes a single maturity gap in TISAX coverage.
type TISAXControlGap struct {
	Control      Control `json:"control"`
	MaturityGap  int     `json:"maturity_gap"`
	CurrentScore int     `json:"current_score"`
}

// TISAXGapAnalysis lists TISAX controls that have not yet reached full maturity.
type TISAXGapAnalysis struct {
	FrameworkID string            `json:"framework_id"`
	TargetScore int               `json:"target_score"`
	Gaps        []TISAXControlGap `json:"gaps"`
}

// Evidence represents a piece of compliance evidence attached to a control.
type Evidence struct {
	ID               string     `json:"id"`
	ControlID        string     `json:"control_id"`
	OrgID            string     `json:"org_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	Source           string     `json:"source"`
	FilePath         string     `json:"-"` // server-side path — never expose in API responses (ARCH-M03)
	FileSize         int64      `json:"file_size,omitempty"`
	CollectorData    []byte     `json:"collector_data,omitempty"`
	Status           string     `json:"status"`
	Version          int        `json:"version"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ExpiryNotifiedAt *time.Time `json:"expiry_notified_at,omitempty"`
	UploadedBy       string     `json:"uploaded_by,omitempty"`
	ReviewedBy       string     `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ChapterMaturity holds per-domain TISAX maturity data within a TISAXMaturitySummary.
type ChapterMaturity struct {
	Domain        string  `json:"domain"`
	AvgScore      float64 `json:"avg_score"`
	TotalControls int     `json:"total_controls"`
	FullyMature   int     `json:"fully_mature"`
	Color         string  `json:"color"` // "green" avg>=2.5, "yellow" avg>=1.5, "red" otherwise
}

// TISAXMaturitySummary summarises TISAX maturity across all domains.
type TISAXMaturitySummary struct {
	AvgScore         float64           `json:"avg_score"`
	ByChapter        []ChapterMaturity `json:"by_chapter"`
	ReadinessPercent float64           `json:"readiness_percent"`
}

// ReadinessReport summarises compliance readiness for a framework.
type ReadinessReport struct {
	FrameworkID    string                `json:"framework_id"`
	FrameworkName  string                `json:"framework_name"`
	TotalControls  int                   `json:"total_controls"`
	Covered        int                   `json:"covered"`
	Partial        int                   `json:"partial"`
	Missing        int                   `json:"missing"`
	ReadinessScore float64               `json:"readiness_score"`
	ByDomain       []DomainScore         `json:"by_domain"`
	TISAXMaturity  *TISAXMaturitySummary `json:"tisax_maturity,omitempty"`
}

// DomainScore holds per-domain readiness data within a ReadinessReport.
type DomainScore struct {
	Domain  string  `json:"domain"`
	Score   float64 `json:"score"`
	Total   int     `json:"total"`
	Covered int     `json:"covered"`
}

// GapAnalysis lists controls that are missing or at-risk evidence.
type GapAnalysis struct {
	FrameworkID string       `json:"framework_id"`
	Gaps        []ControlGap `json:"gaps"`
}

// ControlGap describes a single gap in compliance coverage.
type ControlGap struct {
	Control   Control    `json:"control"`
	Reason    string     `json:"reason"` // "no_evidence", "evidence_expiring", "review_pending"
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Policy represents a managed policy document.
type Policy struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Category      string     `json:"category,omitempty"`
	Status        string     `json:"status"`       // draft | active | archived
	Version       string     `json:"version"`      // user-editable version label, e.g. "1.0"
	VersionNum    int        `json:"version_num"`  // auto-incremented integer version counter
	VersionNote   string     `json:"version_note"` // change summary for the latest version
	LastUpdatedBy string     `json:"last_updated_by"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	NextReviewDue *string    `json:"next_review_due,omitempty"` // date string YYYY-MM-DD
	EffectiveDate *time.Time `json:"effective_date,omitempty"`
	ReviewDate    *time.Time `json:"review_date,omitempty"`
	Owner         string     `json:"owner,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// PolicyVersion holds a historical snapshot of a policy at a given version number.
type PolicyVersion struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	PolicyID    string    `json:"policy_id"`
	Version     int       `json:"version"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	VersionNote string    `json:"version_note"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreatePolicyInput holds validated input for creating a policy.
type CreatePolicyInput struct {
	Title         string     `json:"title"    validate:"required,max=255"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Version       string     `json:"version"`
	EffectiveDate *time.Time `json:"effective_date"`
	ReviewDate    *time.Time `json:"review_date"`
	Owner         string     `json:"owner"`
}

type UpdatePolicyInput struct {
	Title         string     `json:"title"    validate:"required,max=255"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Status        string     `json:"status"   validate:"required,oneof=draft active archived"`
	Version       string     `json:"version"`
	EffectiveDate *time.Time `json:"effective_date"`
	ReviewDate    *time.Time `json:"review_date"`
	Owner         string     `json:"owner"`
	// Versioning fields (Migration 076)
	VersionNote   *string `json:"version_note"    validate:"omitempty,max=500"`
	UpdatedBy     *string `json:"updated_by"      validate:"omitempty,max=200"`
	NextReviewDue *string `json:"next_review_due"`
}

// ControlTask is a user-created implementation step for a compliance control.
type ControlTask struct {
	ID        string    `json:"id"`
	ControlID string    `json:"control_id"`
	OrgID     string    `json:"org_id"`
	Text      string    `json:"text"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateControlTaskInput holds validated input for creating a control task.
type CreateControlTaskInput struct {
	Text string `json:"text" validate:"required,max=500"`
}

// UpdateControlTaskInput holds validated input for toggling a task's completion.
type UpdateControlTaskInput struct {
	Completed bool `json:"completed"`
}

// FrameworkMapping represents a cross-framework control mapping stored in ck_framework_mappings.
type FrameworkMapping struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	SourceControlID string    `json:"source_control_id"`
	TargetControlID string    `json:"target_control_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// ControlMapping represents a row from the global ck_framework_control_mappings table
// after JOIN-resolution to org-specific control UUIDs.
type ControlMapping struct {
	ID                string `json:"id"`
	SourceFramework   string `json:"source_framework"`
	SourceControlCode string `json:"source_control_code"`
	TargetFramework   string `json:"target_framework"`
	TargetControlCode string `json:"target_control_code"`
	MappingType       string `json:"mapping_type"`
	// Resolved fields populated by JOIN at query time.
	TargetControlID     string `json:"target_control_id"`
	TargetControlTitle  string `json:"target_control_title"`
	TargetFrameworkName string `json:"target_framework_name"`
}

// FrameworkPairCountRow is returned by GetFrameworkMappingCounts.
type FrameworkPairCountRow struct {
	FrameworkAName string
	FrameworkBName string
	MappingCount   int
}

// ControlPrerequisiteRow is a row from ck_control_prerequisites.
type ControlPrerequisiteRow struct {
	ControlFramework      string
	ControlCode           string
	PrerequisiteFramework string
	PrerequisiteCode      string
	DependencyType        string
	Rationale             string
}

// MappingResult describes a TISAX control together with its mapped ISO 27001 control and coverage status.
type MappingResult struct {
	TISAXControlID    string `json:"tisax_control_id"`
	TISAXControlTitle string `json:"tisax_control_title"`
	ISOControlID      string `json:"iso_control_id"`
	ISOControlTitle   string `json:"iso_control_title"`
	Covered           bool   `json:"covered"`
}

// GeneratePolicyDraftInput holds validated input for generating a policy draft via AI.
type GeneratePolicyDraftInput struct {
	PolicyType    string `json:"policy_type"    validate:"required"`
	FrameworkID   string `json:"framework_id"`
	OrgName       string `json:"org_name"`
	CustomContext string `json:"custom_context"`
}

// ControlMeasure represents a recommended implementation measure for a compliance control.
type ControlMeasure struct {
	ID          string    `json:"id"`
	ControlID   string    `json:"control_id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Difficulty  string    `json:"difficulty"` // easy | medium | hard
	StepOrder   int       `json:"step_order"`
	IsBuiltin   bool      `json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateMeasureInput holds validated input for creating a control measure.
type CreateMeasureInput struct {
	Title       string `json:"title"       validate:"required,min=3,max=200"`
	Description string `json:"description" validate:"max=2000"`
	Difficulty  string `json:"difficulty"  validate:"required,oneof=easy medium hard"`
	StepOrder   int    `json:"step_order"`
}

// UpdateMeasureInput holds validated input for updating a control measure.
type UpdateMeasureInput struct {
	Title       *string `json:"title"       validate:"omitempty,min=3,max=200"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Difficulty  *string `json:"difficulty"  validate:"omitempty,oneof=easy medium hard"`
	StepOrder   *int    `json:"step_order"`
}

// SoAEntry is a single Statement of Applicability row (moved from handler_soa.go).
type SoAEntry struct {
	ControlID                  string `json:"control_id"`
	FrameworkName              string `json:"framework_name"`
	Domain                     string `json:"domain"`
	Title                      string `json:"title"`
	Applicable                 bool   `json:"applicable"`
	Status                     string `json:"status"`
	JustificationApplicable    string `json:"justification_applicable,omitempty"`
	JustificationNotApplicable string `json:"justification_not_applicable,omitempty"`
}
