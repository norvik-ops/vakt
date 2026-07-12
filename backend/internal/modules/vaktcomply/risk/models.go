// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import "time"

// --- Risk Assessment (FR-CK12) ---

// Risk represents an entry in the ISO 27001 / NIS2 risk register.
type Risk struct {
	ID             string `json:"id"`
	OrgID          string `json:"org_id"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Category       string `json:"category,omitempty"`
	Likelihood     int    `json:"likelihood"` // 1–5
	Impact         int    `json:"impact"`     // 1–5
	RiskScore      int    `json:"risk_score"` // computed: likelihood * impact
	Owner          string `json:"owner,omitempty"`
	Status         string `json:"status"`    // open | mitigated | accepted | closed
	Treatment      string `json:"treatment"` // avoid | mitigate | transfer | accept
	TreatmentNotes string `json:"treatment_notes,omitempty"`
	// Treatment workflow fields (Migration 071)
	TreatmentOption    string     `json:"treatment_option"`
	TreatmentPlan      string     `json:"treatment_plan"`
	TreatmentOwner     string     `json:"treatment_owner"`
	TreatmentDueDate   *time.Time `json:"treatment_due_date"`
	TreatmentStatus    string     `json:"treatment_status"`
	ResidualLikelihood *int       `json:"residual_likelihood"`
	ResidualImpact     *int       `json:"residual_impact"`
	// Residualrisiko-Berechnung (S61-4, Migration 164)
	InherentLikelihood          *int       `json:"inherent_likelihood,omitempty"`
	InherentImpact              *int       `json:"inherent_impact,omitempty"`
	InherentScore               *int       `json:"inherent_score,omitempty"` // computed: InherentLikelihood * InherentImpact
	ResidualScore               *int       `json:"residual_score,omitempty"` // computed: ResidualLikelihood * ResidualImpact
	RiskAcceptedBy              *string    `json:"risk_accepted_by,omitempty"`
	RiskAcceptedAt              *time.Time `json:"risk_accepted_at,omitempty"`
	RiskAcceptanceJustification string     `json:"risk_acceptance_justification"`
	AINarrative                 string     `json:"ai_narrative,omitempty"` // S125 (DB-02): AI-generated risk narrative
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

// ComputeScores calculates InherentScore and ResidualScore from their factors.
func (r *Risk) ComputeScores() {
	if r.InherentLikelihood != nil && r.InherentImpact != nil {
		score := *r.InherentLikelihood * *r.InherentImpact
		r.InherentScore = &score
	}
	if r.ResidualLikelihood != nil && r.ResidualImpact != nil {
		score := *r.ResidualLikelihood * *r.ResidualImpact
		r.ResidualScore = &score
	}
}

// CreateRiskInput holds validated input for creating a risk entry.
type CreateRiskInput struct {
	Title          string `json:"title"       validate:"required,max=255"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Likelihood     int    `json:"likelihood"  validate:"required,min=1,max=5"`
	Impact         int    `json:"impact"      validate:"required,min=1,max=5"`
	Owner          string `json:"owner"`
	Treatment      string `json:"treatment"   validate:"required,oneof=avoid mitigate transfer accept"`
	TreatmentNotes string `json:"treatment_notes"`
}

// UpdateRiskInput holds validated input for updating a risk entry.
type UpdateRiskInput struct {
	Title          string `json:"title"       validate:"required,max=255"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	Likelihood     int    `json:"likelihood"  validate:"required,min=1,max=5"`
	Impact         int    `json:"impact"      validate:"required,min=1,max=5"`
	Owner          string `json:"owner"`
	Status         string `json:"status"      validate:"required,oneof=open mitigated accepted closed"`
	Treatment      string `json:"treatment"   validate:"required,oneof=avoid mitigate transfer accept"`
	TreatmentNotes string `json:"treatment_notes"`
}

// UpdateRiskTreatmentInput holds the treatment workflow fields for PATCH /risks/:id/treatment.
type UpdateRiskTreatmentInput struct {
	TreatmentOption    string  `json:"treatment_option"    validate:"omitempty,oneof=accept mitigate transfer avoid"`
	TreatmentPlan      string  `json:"treatment_plan"      validate:"max=5000"`
	TreatmentOwner     string  `json:"treatment_owner"     validate:"max=200"`
	TreatmentDueDate   *string `json:"treatment_due_date"`
	TreatmentStatus    string  `json:"treatment_status"    validate:"omitempty,oneof=pending in_progress implemented verified"`
	ResidualLikelihood *int    `json:"residual_likelihood" validate:"omitempty,min=1,max=5"`
	ResidualImpact     *int    `json:"residual_impact"     validate:"omitempty,min=1,max=5"`
}

// UpdateRiskResidualInput holds inherent and residual likelihood/impact factors (S61-4).
type UpdateRiskResidualInput struct {
	InherentLikelihood *int `json:"inherent_likelihood,omitempty" validate:"omitempty,min=1,max=5"`
	InherentImpact     *int `json:"inherent_impact,omitempty"     validate:"omitempty,min=1,max=5"`
	ResidualLikelihood *int `json:"residual_likelihood,omitempty" validate:"omitempty,min=1,max=5"`
	ResidualImpact     *int `json:"residual_impact,omitempty"     validate:"omitempty,min=1,max=5"`
}

// AcceptRiskInput holds the justification for formally accepting a risk (S61-4).
type AcceptRiskInput struct {
	Justification string `json:"justification" validate:"required"`
}

// --- CAPA (Corrective and Preventive Actions) ---

// CAPA represents a corrective or preventive action record.
type CAPA struct {
	ID               string     `json:"id"`
	OrgID            string     `json:"org_id"`
	SourceType       string     `json:"source_type"`
	SourceID         string     `json:"source_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	RootCause        string     `json:"root_cause"`
	ActionPlan       string     `json:"action_plan"`
	AssigneeEmail    string     `json:"assignee_email"`
	DueDate          *time.Time `json:"due_date"`
	Priority         string     `json:"priority"`
	Status           string     `json:"status"`
	VerificationNote string     `json:"verification_note"`
	ClosedAt         *time.Time `json:"closed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// S61-3: NC/CA fields (Migration 163)
	NCClassification       *string    `json:"nc_classification,omitempty"`
	ImmediateContainment   string     `json:"immediate_containment"`
	SimilarNCsAssessed     *bool      `json:"similar_ncs_assessed,omitempty"`
	SimilarNCsNotes        string     `json:"similar_ncs_notes"`
	EffectivenessCheckDate *string    `json:"effectiveness_check_date,omitempty"`
	EffectivenessConfirmed *bool      `json:"effectiveness_confirmed,omitempty"`
	EffectivenessCheckedAt *time.Time `json:"effectiveness_checked_at,omitempty"`
	EffectivenessCheckedBy *string    `json:"effectiveness_checked_by,omitempty"`
	EffectivenessEvidence  string     `json:"effectiveness_evidence"`
}

// CreateCAPAInput holds validated input for creating a CAPA.
type CreateCAPAInput struct {
	SourceType    string  `json:"source_type"    validate:"required,oneof=audit incident risk manual"`
	SourceID      string  `json:"source_id"`
	Title         string  `json:"title"          validate:"required,min=3,max=300"`
	Description   string  `json:"description"    validate:"max=3000"`
	AssigneeEmail string  `json:"assignee_email" validate:"omitempty,email"`
	DueDate       *string `json:"due_date"`
	Priority      string  `json:"priority"       validate:"omitempty,oneof=low medium high critical"`
}

// UpdateCAPAInput holds validated input for updating a CAPA.
type UpdateCAPAInput struct {
	Title            *string `json:"title"             validate:"omitempty,min=3,max=300"`
	Description      *string `json:"description"       validate:"omitempty,max=3000"`
	RootCause        *string `json:"root_cause"        validate:"omitempty,max=3000"`
	ActionPlan       *string `json:"action_plan"       validate:"omitempty,max=5000"`
	AssigneeEmail    *string `json:"assignee_email"    validate:"omitempty,email"`
	DueDate          *string `json:"due_date"`
	Priority         *string `json:"priority"          validate:"omitempty,oneof=low medium high critical"`
	Status           *string `json:"status"            validate:"omitempty,oneof=open in_progress implemented verified closed"`
	VerificationNote *string `json:"verification_note" validate:"omitempty,max=3000"`
}

// BulkUpdateCAPAsInput holds input for PATCH /vaktcomply/capas/bulk.
type BulkUpdateCAPAsInput struct {
	IDs    []string `json:"ids"    validate:"required,min=1,max=100"`
	Status string   `json:"status" validate:"required,oneof=open in_progress implemented verified closed"`
}

// CAPANCFields holds the ISO 9001 / ISO 27001 NC root-cause and effectiveness fields for a CAPA.
type CAPANCFields struct {
	NCClassification       *string    `json:"nc_classification,omitempty"`
	ImmediateContainment   string     `json:"immediate_containment"`
	RootCause              string     `json:"root_cause"`
	SimilarNCsAssessed     *bool      `json:"similar_ncs_assessed,omitempty"`
	SimilarNCsNotes        string     `json:"similar_ncs_notes"`
	EffectivenessCheckDate *string    `json:"effectiveness_check_date,omitempty"`
	EffectivenessConfirmed *bool      `json:"effectiveness_confirmed,omitempty"`
	EffectivenessCheckedAt *time.Time `json:"effectiveness_checked_at,omitempty"`
	EffectivenessCheckedBy *string    `json:"effectiveness_checked_by,omitempty"`
	EffectivenessEvidence  string     `json:"effectiveness_evidence"`
}

// EffectivenessCheckInput holds the payload for completing an effectiveness check on a CAPA.
type EffectivenessCheckInput struct {
	Confirmed    bool   `json:"confirmed"     validate:"required"`
	EvidenceNote string `json:"evidence_note"`
}

// --- DORA IKT-Drittanbieter-Register (Art. 28-44 / S38-1) ---

// DORAThirdParty represents an IKT third-party service provider in the DORA register.
type DORAThirdParty struct {
	ID                 string    `json:"id"`
	OrgID              string    `json:"org_id"`
	Name               string    `json:"name"`
	ServiceType        string    `json:"service_type"`
	Criticality        string    `json:"criticality"`
	ContractStart      *string   `json:"contract_start,omitempty"` // ISO date string
	ContractEnd        *string   `json:"contract_end,omitempty"`   // ISO date string
	SLARTOHours        *int      `json:"sla_rto_hours,omitempty"`
	SLAAvailability    *float64  `json:"sla_availability,omitempty"`
	HasSubcontractors  bool      `json:"has_subcontractors"`
	SubcontractorNames string    `json:"subcontractor_names,omitempty"`
	DataLocation       string    `json:"data_location"`
	ExitStrategy       bool      `json:"exit_strategy"`
	ExitNotes          string    `json:"exit_notes,omitempty"`
	Notes              string    `json:"notes,omitempty"`
	CreatedBy          *string   `json:"created_by,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	// Control IDs linked via dora_third_party_controls — populated on get, not list
	ControlIDs []string `json:"control_ids,omitempty"`
}

// CreateDORAThirdPartyInput holds validated input for creating a third party entry.
type CreateDORAThirdPartyInput struct {
	Name               string   `json:"name"                validate:"required,min=1,max=255"`
	ServiceType        string   `json:"service_type"        validate:"required,oneof=IT-Outsourcing Cloud SaaS Netzwerk Sonstiges"`
	Criticality        string   `json:"criticality"         validate:"required,oneof=kritisch wichtig unkritisch"`
	ContractStart      *string  `json:"contract_start"`
	ContractEnd        *string  `json:"contract_end"`
	SLARTOHours        *int     `json:"sla_rto_hours"`
	SLAAvailability    *float64 `json:"sla_availability"`
	HasSubcontractors  bool     `json:"has_subcontractors"`
	SubcontractorNames string   `json:"subcontractor_names"`
	DataLocation       string   `json:"data_location"       validate:"required,oneof=EU Non-EU Mixed"`
	ExitStrategy       bool     `json:"exit_strategy"`
	ExitNotes          string   `json:"exit_notes"`
	Notes              string   `json:"notes"`
}

// UpdateDORAThirdPartyInput holds validated input for updating a third party entry.
type UpdateDORAThirdPartyInput struct {
	Name               string   `json:"name"                validate:"required,min=1,max=255"`
	ServiceType        string   `json:"service_type"        validate:"required,oneof=IT-Outsourcing Cloud SaaS Netzwerk Sonstiges"`
	Criticality        string   `json:"criticality"         validate:"required,oneof=kritisch wichtig unkritisch"`
	ContractStart      *string  `json:"contract_start"`
	ContractEnd        *string  `json:"contract_end"`
	SLARTOHours        *int     `json:"sla_rto_hours"`
	SLAAvailability    *float64 `json:"sla_availability"`
	HasSubcontractors  bool     `json:"has_subcontractors"`
	SubcontractorNames string   `json:"subcontractor_names"`
	DataLocation       string   `json:"data_location"       validate:"required,oneof=EU Non-EU Mixed"`
	ExitStrategy       bool     `json:"exit_strategy"`
	ExitNotes          string   `json:"exit_notes"`
	Notes              string   `json:"notes"`
}

// ── S60: Schutzbedarfsfeststellung ────────────────────────────────────────────

// ProtectionNeedAssessment represents a BSI Schutzbedarfsfeststellung record.
type ProtectionNeedAssessment struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	Name            string     `json:"name"`
	ObjectType      string     `json:"object_type"`
	ObjectName      string     `json:"object_name"`
	Confidentiality string     `json:"confidentiality"`
	Integrity       string     `json:"integrity"`
	Availability    string     `json:"availability"`
	Overall         string     `json:"overall"`
	Status          string     `json:"status"`
	VBAssetID       *string    `json:"vb_asset_id,omitempty"` // soft link to vb_assets, no FK
	FinalizedAt     *time.Time `json:"finalized_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateProtectionNeedInput is the request body for creating a new assessment.
type CreateProtectionNeedInput struct {
	Name       string `json:"name"        validate:"required"`
	ObjectType string `json:"object_type" validate:"required,oneof=process system information location"`
	ObjectName string `json:"object_name" validate:"required"`
}

// UpdateProtectionNeedInput is the request body for rating C/I/A.
type UpdateProtectionNeedInput struct {
	Confidentiality string `json:"confidentiality" validate:"required,oneof=normal hoch sehr_hoch"`
	Integrity       string `json:"integrity"       validate:"required,oneof=normal hoch sehr_hoch"`
	Availability    string `json:"availability"    validate:"required,oneof=normal hoch sehr_hoch"`
}

// --- Control Exceptions ---

// ControlException represents a formal waiver / exception for a compliance control.
type ControlException struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	ControlID    string     `json:"control_id"`
	Title        string     `json:"title"`
	Reason       string     `json:"reason"`
	RiskAccepted string     `json:"risk_accepted"`
	ApprovedBy   string     `json:"approved_by,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Status       string     `json:"status"` // active | expired | revoked
	CreatedBy    string     `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CreateControlExceptionInput holds validated input for creating a new exception.
type CreateControlExceptionInput struct {
	Title        string     `json:"title"         validate:"required,max=255"`
	Reason       string     `json:"reason"        validate:"required,max=2000"`
	RiskAccepted string     `json:"risk_accepted" validate:"required,max=2000"`
	ApprovedBy   string     `json:"approved_by"   validate:"max=255"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// UpdateControlExceptionInput holds validated input for updating an existing exception.
type UpdateControlExceptionInput struct {
	Title        string     `json:"title"         validate:"omitempty,max=255"`
	Reason       string     `json:"reason"        validate:"omitempty,max=2000"`
	RiskAccepted string     `json:"risk_accepted" validate:"omitempty,max=2000"`
	ApprovedBy   string     `json:"approved_by"   validate:"omitempty,max=255"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Status       string     `json:"status"        validate:"omitempty,oneof=active expired revoked"`
}
