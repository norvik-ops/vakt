// Package hr provides HTTP handlers and business logic for the HR onboarding/offboarding module.
package hr

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
