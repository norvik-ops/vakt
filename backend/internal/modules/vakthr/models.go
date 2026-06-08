// Package hr provides HTTP handlers and business logic for the HR onboarding/offboarding module.
package vakthr

import "time"

// Employee represents a member of staff tracked in the HR module.
type Employee struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Email      string    `json:"email"`
	Department string    `json:"department,omitempty"`
	Role       string    `json:"role,omitempty"`
	StartDate  *string   `json:"start_date,omitempty"`
	EndDate    *string   `json:"end_date,omitempty"`
	Status     string    `json:"status"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateEmployeeInput is the request body for creating a new employee.
type CreateEmployeeInput struct {
	FirstName  string `json:"first_name"  validate:"required"`
	LastName   string `json:"last_name"   validate:"required"`
	Email      string `json:"email"       validate:"required,email"`
	Department string `json:"department"`
	Role       string `json:"role"`
	StartDate  string `json:"start_date"`
	Notes      string `json:"notes"`
}

// UpdateEmployeeInput is the request body for updating an existing employee.
type UpdateEmployeeInput struct {
	FirstName  string `json:"first_name"  validate:"required"`
	LastName   string `json:"last_name"   validate:"required"`
	Department string `json:"department"`
	Role       string `json:"role"`
	EndDate    string `json:"end_date"`
	Status     string `json:"status"      validate:"required,oneof=active offboarding terminated"`
	Notes      string `json:"notes"`
}

// ChecklistItem is a single step within a checklist template.
type ChecklistItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// Checklist is an onboarding or offboarding checklist template.
type Checklist struct {
	ID        string          `json:"id"`
	OrgID     string          `json:"org_id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Items     []ChecklistItem `json:"items"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CreateChecklistInput is the request body for creating a checklist template.
type CreateChecklistInput struct {
	Type  string          `json:"type"  validate:"required,oneof=onboarding offboarding"`
	Name  string          `json:"name"  validate:"required,max=255"`
	Items []ChecklistItem `json:"items"`
}

// ChecklistRun tracks the execution of a checklist for a specific employee.
type ChecklistRun struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	EmployeeID     string     `json:"employee_id"`
	ChecklistID    string     `json:"checklist_id"`
	Status         string     `json:"status"`
	CompletedItems []string   `json:"completed_items"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// StartChecklistRunInput is the request body for starting a checklist run.
type StartChecklistRunInput struct {
	EmployeeID  string `json:"employee_id"  validate:"required"`
	ChecklistID string `json:"checklist_id" validate:"required"`
}

// UpdateChecklistRunInput is the request body for updating a checklist run's progress.
type UpdateChecklistRunInput struct {
	CompletedItems []string `json:"completed_items"`
	Status         string   `json:"status" validate:"required,oneof=in_progress completed"`
}

// RunEvent records a single step completion within a checklist run. Used as the
// audit trail (who completed which step, when) for compliance evidence.
type RunEvent struct {
	ID          string    `json:"id"`
	RunID       string    `json:"run_id"`
	OrgID       string    `json:"org_id"`
	StepID      string    `json:"step_id"`
	CompletedBy string    `json:"completed_by"`
	CompletedAt time.Time `json:"completed_at"`
}

// ── S60: Berechtigungskonzept ─────────────────────────────────────────────────

// AccessConcept represents a Berechtigungskonzept document.
type AccessConcept struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	Title          string    `json:"title"`
	Scope          string    `json:"scope"`
	Owner          string    `json:"owner"`
	CurrentVersion int32     `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AccessRole represents a single role definition within an AccessConcept.
type AccessRole struct {
	ID                   string    `json:"id"`
	ConceptID            string    `json:"concept_id"`
	OrgID                string    `json:"org_id"`
	RoleName             string    `json:"role_name"`
	SystemName           string    `json:"system_name"`
	AccessLevel          string    `json:"access_level"`
	Justification        string    `json:"justification"`
	ReviewIntervalMonths int32     `json:"review_interval_months"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// AccessConceptVersionSummary is the slim row returned when listing version snapshots.
type AccessConceptVersionSummary struct {
	ID            string    `json:"id"`
	ConceptID     string    `json:"concept_id"`
	VersionNumber int32     `json:"version_number"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateAccessConceptInput is the request body for creating an access concept.
type CreateAccessConceptInput struct {
	Title string `json:"title" validate:"required"`
	Scope string `json:"scope"`
	Owner string `json:"owner"`
}

// UpdateAccessConceptInput is the request body for updating an access concept.
type UpdateAccessConceptInput struct {
	Title string `json:"title" validate:"required"`
	Scope string `json:"scope"`
	Owner string `json:"owner"`
}

// CreateAccessRoleInput is the request body for adding a role to an access concept.
type CreateAccessRoleInput struct {
	RoleName             string `json:"role_name"              validate:"required"`
	SystemName           string `json:"system_name"            validate:"required"`
	AccessLevel          string `json:"access_level"           validate:"required,oneof=read write admin no_access"`
	Justification        string `json:"justification"`
	ReviewIntervalMonths int32  `json:"review_interval_months"`
}

// UpdateAccessRoleInput is the request body for updating a role definition.
type UpdateAccessRoleInput struct {
	RoleName             string `json:"role_name"              validate:"required"`
	SystemName           string `json:"system_name"            validate:"required"`
	AccessLevel          string `json:"access_level"           validate:"required,oneof=read write admin no_access"`
	Justification        string `json:"justification"`
	ReviewIntervalMonths int32  `json:"review_interval_months"`
}
