package auditor

import "time"

// AuditorInvite represents an auditor invitation record.
type AuditorInvite struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Email      string     `json:"email"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateInviteInput holds validated input for creating an auditor invite.
type CreateInviteInput struct {
	Email     string `json:"email"      validate:"required,email"`
	ExpiresIn int    `json:"expires_in" validate:"required,min=1,max=90"` // days
}

// AuditorClaims holds identity data for an active auditor session.
type AuditorClaims struct {
	OrgID        string    `json:"org_id"`
	AuditorEmail string    `json:"auditor_email"`
	SessionID    string    `json:"session_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}
