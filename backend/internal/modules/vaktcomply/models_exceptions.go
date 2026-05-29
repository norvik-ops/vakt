package vaktcomply

import "time"

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
