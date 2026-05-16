package usermgmt

import "time"

// UserWithRole represents an organisation member with their simple role.
type UserWithRole struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Invitation represents a pending or accepted team invitation.
type Invitation struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	InvitedBy  string     `json:"invited_by"`
	AcceptedAt *time.Time `json:"accepted_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// InviteInput is the validated input for creating an invitation.
type InviteInput struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=admin editor viewer"`
}

// UpdateRoleInput is the validated input for changing a user's role.
type UpdateRoleInput struct {
	Role string `json:"role" validate:"required,oneof=admin editor viewer"`
}

// AcceptInviteInput is the validated input for accepting an invitation and
// creating a new user account.
type AcceptInviteInput struct {
	Token    string `json:"token"    validate:"required"`
	Name     string `json:"name"     validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}
