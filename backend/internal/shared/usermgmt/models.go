package usermgmt

import "time"

// UserWithRole represents an organisation member with their role.
//
// Two fields, because Vakt has two role columns and ADR-0077 names exactly one
// of them authoritative:
//
//   - PlatformRole is `org_members.role_id -> roles.name` — the role the token
//     claim carries and every auth.RequireRole guard checks, usermgmt.requireAdmin
//     included since ESK-13. Use this one.
//   - Role is the denormalised `users.role` cache (admin/editor/viewer). It
//     authorises nothing: requireAdmin was its last reader with a decision to
//     make, and it now reads org_members like everything else (REV-ESK13 §2.1).
//     It also cannot express AuditorReadOnly or InternalAuditor (both collapse to
//     "viewer"), so a UI that renders it shows an internal auditor as a plain
//     viewer. Kept for compatibility with clients written before PlatformRole
//     existed.
type UserWithRole struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	PlatformRole string    `json:"platform_role"`
	CreatedAt    time.Time `json:"created_at"`
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
//
// R1-14cA-11: neighbouring routes spoke incompatible role vocabularies. The
// invitation took only admin|editor|viewer while the role change (and the
// account-creation route next to it) speak the platform names — so a client that
// knew one set failed on the other. The list here is the same as
// UpdateRoleInput's, and invite_role_parity_test.go keeps it that way.
//
// Widening it is only safe because AcceptInvitation now resolves BOTH role
// columns through assignableRoles. It used to write the invitation's raw string
// into users.role, which carries CHECK (role IN ('admin','editor','viewer')) —
// an invitation for "Admin" would have failed at 'accept' time, after the
// invitee clicked the link and typed a password.
type InviteInput struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly InternalAuditor admin editor viewer"`
}

// UpdateRoleInput is the validated input for changing a user's role.
//
// The accepted vocabulary is the platform role names (roles.name) plus the three
// lowercase names this endpoint accepted before — those are kept as aliases so
// clients written against the old contract keep working. The list must stay in
// sync with assignableRoles in service.go; a value the validator lets through
// but the map does not know is rejected there rather than silently downgraded.
//
// AuditorReadOnly and InternalAuditor are here because before ESK-13 the platform
// offered no way to grant them at all. org_members.role_id had nine INSERT sites
// in non-test code (seven outside cmd/seed and demoseed) and no UPDATE site, so a
// role could only be set while the account was created — and even there only
// AuditorReadOnly was reachable: POST /admin/users validates
// `oneof=Admin SecurityAnalyst Viewer AuditorReadOnly` (admin/service.go:35,42),
// which never accepted InternalAuditor. ADR-0055 requires an admin to assign
// InternalAuditor explicitly; this endpoint is where that became possible.
type UpdateRoleInput struct {
	Role string `json:"role" validate:"required,oneof=Admin SecurityAnalyst Viewer AuditorReadOnly InternalAuditor admin editor viewer"`
}

// AcceptInviteInput is the validated input for accepting an invitation and
// creating a new user account.
type AcceptInviteInput struct {
	Token    string `json:"token"    validate:"required"`
	Name     string `json:"name"     validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=10,max=72"`
}
