// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"encoding/json"
	"time"
)

// --- Internal Audit Records (FR-CK15) ---

// AuditRecord represents an internal audit.
type AuditRecord struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Title           string    `json:"title"`
	Scope           string    `json:"scope,omitempty"`
	Auditor         string    `json:"auditor,omitempty"`
	AuditDate       time.Time `json:"audit_date"`
	Status          string    `json:"status"` // planned | in_progress | completed
	Findings        string    `json:"findings,omitempty"`
	Recommendations string    `json:"recommendations,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateAuditRecordInput holds validated input for creating an audit record.
type CreateAuditRecordInput struct {
	Title           string    `json:"title"      validate:"required,max=255"`
	Scope           string    `json:"scope"`
	Auditor         string    `json:"auditor"`
	AuditDate       time.Time `json:"audit_date" validate:"required"`
	Findings        string    `json:"findings"`
	Recommendations string    `json:"recommendations"`
}

// UpdateAuditRecordInput holds validated input for updating an audit record.
type UpdateAuditRecordInput struct {
	Title           string    `json:"title"      validate:"required,max=255"`
	Scope           string    `json:"scope"`
	Auditor         string    `json:"auditor"`
	AuditDate       time.Time `json:"audit_date" validate:"required"`
	Status          string    `json:"status"     validate:"required,oneof=planned in_progress completed"`
	Findings        string    `json:"findings"`
	Recommendations string    `json:"recommendations"`
}

// --- Audit Program (ISO 27001 Clause 9.2) ---

// AuditPlan is a yearly audit planning document for ISO 27001 Clause 9.2.
type AuditPlan struct {
	ID            string  `json:"id"`
	OrgID         string  `json:"org_id"`
	Year          int     `json:"year"`
	Scope         string  `json:"scope,omitempty"`
	ResponsibleID *string `json:"responsible_id,omitempty"`
	Status        string  `json:"status"`
	Notes         string  `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// AuditProgramAudit is an individual audit within an audit plan.
type AuditProgramAudit struct {
	ID            string   `json:"id"`
	OrgID         string   `json:"org_id"`
	AuditPlanID   *string  `json:"audit_plan_id,omitempty"`
	Title         string   `json:"title"`
	AuditType     string   `json:"audit_type"`
	Scope         string   `json:"scope"`
	Methodology   string   `json:"methodology"`
	PlannedDate   string   `json:"planned_date"`
	ActualDate    *string  `json:"actual_date,omitempty"`
	LeadAuditorID *string  `json:"lead_auditor_id,omitempty"`
	AuditorIDs    []string `json:"auditor_ids"`
	SupplierID    *string  `json:"supplier_id,omitempty"`
	Status        string   `json:"status"`
	AuditReport   string   `json:"audit_report,omitempty"`
	FindingsCount int      `json:"findings_count"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

// AuditFinding is a finding recorded during an audit.
type AuditFinding struct {
	ID                string  `json:"id"`
	OrgID             string  `json:"org_id"`
	AuditID           string  `json:"audit_id"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	Severity          string  `json:"severity"`
	AffectedControlID *string `json:"affected_control_id,omitempty"`
	CAPAid            *string `json:"capa_id,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

// AuditProgramSummary holds aggregate stats for the audit program dashboard.
type AuditProgramSummary struct {
	AuditsPlannedThisYear  int `json:"audits_planned_this_year"`
	AuditsCompleted        int `json:"audits_completed"`
	OpenFindings           int `json:"open_findings"`
	OverdueCAPAsFromAudits int `json:"overdue_capas_from_audits"`
}

// CreateAuditPlanInput holds validated input for a new audit plan.
type CreateAuditPlanInput struct {
	Year          int     `json:"year"  validate:"required,min=2000,max=2100"`
	Scope         string  `json:"scope,omitempty"`
	ResponsibleID *string `json:"responsible_id,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// CreateAuditProgramAuditInput holds validated input for an individual audit.
type CreateAuditProgramAuditInput struct {
	AuditPlanID   *string  `json:"audit_plan_id,omitempty"`
	Title         string   `json:"title"       validate:"required,max=300"`
	AuditType     string   `json:"audit_type"  validate:"required,oneof=isms_internal compliance_check supplier_audit process_audit"`
	Scope         string   `json:"scope"       validate:"required,max=5000"`
	Methodology   string   `json:"methodology" validate:"omitempty,oneof=document_review interview technical_check combined"`
	PlannedDate   string   `json:"planned_date" validate:"required"`
	LeadAuditorID *string  `json:"lead_auditor_id,omitempty"`
	AuditorIDs    []string `json:"auditor_ids,omitempty"`
	SupplierID    *string  `json:"supplier_id,omitempty"`
}

// CompleteAuditInput holds the audit report and actual completion date.
type CompleteAuditInput struct {
	AuditReport string `json:"audit_report" validate:"required,min=10,max=50000"`
	ActualDate  string `json:"actual_date"  validate:"required"`
}

// CreateAuditFindingInput holds validated input for a finding.
type CreateAuditFindingInput struct {
	Title             string  `json:"title"       validate:"required,max=300"`
	Description       string  `json:"description" validate:"required,max=10000"`
	Severity          string  `json:"severity"    validate:"required,oneof=major_nc minor_nc observation ofi"`
	AffectedControlID *string `json:"affected_control_id,omitempty"`
}

// --- Management Review (ISO 27001 Clause 9.3) ---

// ManagementReview represents an ISO 27001 Clause 9.3 management review.
type ManagementReview struct {
	ID                    string          `json:"id"`
	OrgID                 string          `json:"org_id"`
	ReviewDate            string          `json:"review_date"`
	ReviewType            string          `json:"review_type"`
	ParticipantIDs        json.RawMessage `json:"participant_ids"`
	Status                string          `json:"status"`
	AuditFindingsSummary  string          `json:"audit_findings_summary"`
	IncidentSummary       string          `json:"incident_summary"`
	RiskStatusSummary     string          `json:"risk_status_summary"`
	PreviousActionsStatus string          `json:"previous_actions_status"`
	KPISnapshot           json.RawMessage `json:"kpi_snapshot,omitempty"`
	ContextChanges        string          `json:"context_changes"`
	CustomerFeedback      string          `json:"customer_feedback"`
	ImprovementDecisions  json.RawMessage `json:"improvement_decisions"`
	ResourceDecisions     string          `json:"resource_decisions"`
	ISMSChanges           string          `json:"isms_changes"`
	NextReviewDate        *string         `json:"next_review_date,omitempty"`
	ApprovedBy            *string         `json:"approved_by,omitempty"`
	ApprovedAt            *time.Time      `json:"approved_at,omitempty"`
	CreatedBy             string          `json:"created_by"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// CreateManagementReviewInput holds validated input for creating a new management review.
type CreateManagementReviewInput struct {
	ReviewDate     string          `json:"review_date"    validate:"required"`
	ReviewType     string          `json:"review_type"    validate:"required,oneof=annual extraordinary"`
	ParticipantIDs json.RawMessage `json:"participant_ids"`
}

// UpdateManagementReviewInputsInput holds input-phase fields for a management review.
type UpdateManagementReviewInputsInput struct {
	AuditFindingsSummary  string          `json:"audit_findings_summary"`
	IncidentSummary       string          `json:"incident_summary"`
	RiskStatusSummary     string          `json:"risk_status_summary"`
	PreviousActionsStatus string          `json:"previous_actions_status"`
	KPISnapshot           json.RawMessage `json:"kpi_snapshot,omitempty"`
	ContextChanges        string          `json:"context_changes"`
	CustomerFeedback      string          `json:"customer_feedback"`
}

// UpdateManagementReviewOutputsInput holds output-phase fields for a management review.
type UpdateManagementReviewOutputsInput struct {
	ImprovementDecisions json.RawMessage `json:"improvement_decisions"`
	ResourceDecisions    string          `json:"resource_decisions"`
	ISMSChanges          string          `json:"isms_changes"`
	NextReviewDate       *string         `json:"next_review_date,omitempty"`
}
