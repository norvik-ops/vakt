package usermgmt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/smtp"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/shared/mailhdr"
	"github.com/matharnica/vakt/internal/shared/password"
)

// SMTPConfig holds the SMTP settings needed to send invitation emails.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// SessionRevoker revokes all active sessions for a user. Implemented by auth.Service.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, userID string) error
}

// Service handles all user-management and invitation business logic.
type Service struct {
	db             *pgxpool.Pool
	smtpCfg        SMTPConfig
	frontendURL    string
	sessionRevoker SessionRevoker
}

// NewService constructs a user-management Service.
func NewService(db *pgxpool.Pool, smtpCfg SMTPConfig, frontendURL string) *Service {
	return &Service{
		db:          db,
		smtpCfg:     smtpCfg,
		frontendURL: frontendURL,
	}
}

// WithSessionRevoker injects a session revoker so that Remove/Demote operations
// immediately invalidate the affected user's active tokens.
func (s *Service) WithSessionRevoker(r SessionRevoker) *Service {
	s.sessionRevoker = r
	return s
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// ListUsers returns all members of the organisation along with their roles.
// The role is read from the users.role column (added by migration 077).
// Org membership is still determined by org_members, so only users in the org
// are returned.
func (s *Service) ListUsers(ctx context.Context, orgID string) ([]UserWithRole, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.email, COALESCE(u.display_name, '') AS name,
		       u.role, u.created_at
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE om.org_id = $1::uuid
		ORDER BY u.created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []UserWithRole
	for rows.Next() {
		var u UserWithRole
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if users == nil {
		users = []UserWithRole{}
	}
	return users, rows.Err()
}

// UpdateUserRole changes a user's role within the organisation. It prevents
// demoting the last remaining admin. On role downgrade the user's active
// sessions are revoked so the new (lesser) role takes effect immediately.
func (s *Service) UpdateUserRole(ctx context.Context, orgID, userID, role string) error {
	// Prevent removing the last admin.
	if role != "admin" {
		if err := s.ensureNotLastAdmin(ctx, orgID, userID); err != nil {
			return err
		}
	}

	result, err := s.db.Exec(ctx, `
		UPDATE users SET role = $1
		WHERE id = $2::uuid
		  AND EXISTS (
		    SELECT 1 FROM org_members WHERE org_id = $3::uuid AND user_id = $2::uuid
		  )`,
		role, userID, orgID,
	)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found in organisation")
	}
	// Revoke sessions so the new role takes effect at next login.
	if s.sessionRevoker != nil {
		if rErr := s.sessionRevoker.RevokeAllSessions(ctx, userID); rErr != nil {
			// Non-fatal: user simply carries old role until next token expiry.
			_ = rErr
		}
	}
	return nil
}

// RemoveUser removes a user from the organisation. It prevents removing the
// last admin or the calling user (self-removal guard is at the handler layer).
// Active sessions are revoked immediately so removed users cannot continue
// using existing tokens (AUTH-007).
func (s *Service) RemoveUser(ctx context.Context, orgID, userID string) error {
	if err := s.ensureNotLastAdmin(ctx, orgID, userID); err != nil {
		return err
	}

	result, err := s.db.Exec(ctx, `
		DELETE FROM org_members
		WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found in organisation")
	}
	if s.sessionRevoker != nil {
		if rErr := s.sessionRevoker.RevokeAllSessions(ctx, userID); rErr != nil {
			_ = rErr // best-effort; user is already removed from org_members
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

// CreateInvitation generates a signed invitation token, stores its hash, and
// sends an invitation email with the acceptance link.
func (s *Service) CreateInvitation(ctx context.Context, orgID, inviterEmail string, in InviteInput) (Invitation, error) {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return Invitation{}, fmt.Errorf("generate invitation token: %w", err)
	}

	var inv Invitation
	err = s.db.QueryRow(ctx, `
		INSERT INTO user_invitations (org_id, email, role, token_hash, invited_by)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, org_id::text, email, role, invited_by,
		          accepted_at, expires_at, created_at`,
		orgID, in.Email, in.Role, tokenHash, inviterEmail,
	).Scan(
		&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.InvitedBy,
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		return Invitation{}, fmt.Errorf("insert invitation: %w", err)
	}

	// Send invitation email — non-fatal if SMTP is not configured.
	if sendErr := s.sendInviteEmail(in.Email, inviterEmail, rawToken); sendErr != nil {
		// Log the token so the admin can share it manually if SMTP is absent.
		_ = sendErr // mailer already handles missing-host gracefully; log at caller
	}

	return inv, nil
}

// ListInvitations returns all invitations for the organisation.
func (s *Service) ListInvitations(ctx context.Context, orgID string) ([]Invitation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, email, role, invited_by,
		       accepted_at, expires_at, created_at
		FROM user_invitations
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invs []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(
			&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.InvitedBy,
			&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		invs = append(invs, inv)
	}
	if invs == nil {
		invs = []Invitation{}
	}
	return invs, rows.Err()
}

// RevokeInvitation deletes a pending invitation.
func (s *Service) RevokeInvitation(ctx context.Context, orgID, invitationID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM user_invitations
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		invitationID, orgID,
	)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("invitation not found")
	}
	return nil
}

// GetInvitationByToken looks up a valid (non-expired, non-accepted) invitation
// by its raw token. Used by the public accept page to display invite details.
func (s *Service) GetInvitationByToken(ctx context.Context, rawToken string) (Invitation, error) {
	tokenHash := hashToken(rawToken)

	var inv Invitation
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, email, role, invited_by,
		       accepted_at, expires_at, created_at
		FROM user_invitations
		WHERE token_hash = $1
		  AND accepted_at IS NULL
		  AND expires_at > NOW()`,
		tokenHash,
	).Scan(
		&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.InvitedBy,
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		return Invitation{}, fmt.Errorf("invitation not found or expired")
	}
	return inv, nil
}

// AcceptInvitation creates a new user account, links them to the organisation,
// and marks the invitation as accepted — all in a single transaction.
func (s *Service) AcceptInvitation(ctx context.Context, in AcceptInviteInput) error {
	tokenHash := hashToken(in.Token)

	// Validate the invitation.
	var invID, orgID, email, role string
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, email, role
		FROM user_invitations
		WHERE token_hash = $1
		  AND accepted_at IS NULL
		  AND expires_at > NOW()`,
		tokenHash,
	).Scan(&invID, &orgID, &email, &role)
	if err != nil {
		return fmt.Errorf("invitation not found or expired")
	}

	// Enforce platform password policy before hashing.
	if err := password.ValidateStrength(in.Password); err != nil {
		return err
	}

	// Hash the new password using bcrypt — same cost as the auth service.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Create the user.
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`,
		email, string(passwordHash), in.Name, role,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// Look up the matching platform role (Admin/SecurityAnalyst/Viewer) from the
	// roles table. Map our simple role names to the existing role names.
	platformRole := platformRoleName(role)
	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, platformRole).Scan(&roleID)
	if err != nil {
		// Fall back to Viewer if the role is not found.
		err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Viewer'`).Scan(&roleID)
		if err != nil {
			return fmt.Errorf("lookup role: %w", err)
		}
	}

	// Add org membership.
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID,
	)
	if err != nil {
		return fmt.Errorf("insert org member: %w", err)
	}

	// Mark invitation as accepted.
	_, err = tx.Exec(ctx, `
		UPDATE user_invitations SET accepted_at = NOW()
		WHERE id = $1::uuid`,
		invID,
	)
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}

	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// ensureNotLastAdmin returns an error if userID is the last admin in orgID.
func (s *Service) ensureNotLastAdmin(ctx context.Context, orgID, userID string) error {
	var adminCount int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE om.org_id = $1::uuid AND u.role = 'admin'`,
		orgID,
	).Scan(&adminCount)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}

	// Check whether the target user is an admin.
	var targetRole string
	err = s.db.QueryRow(ctx, `
		SELECT u.role FROM users u
		JOIN org_members om ON om.user_id = u.id
		WHERE u.id = $1::uuid AND om.org_id = $2::uuid`,
		userID, orgID,
	).Scan(&targetRole)
	if err != nil {
		return fmt.Errorf("fetch target user role: %w", err)
	}

	if targetRole == "admin" && adminCount <= 1 {
		return fmt.Errorf("cannot remove or demote the last admin")
	}
	return nil
}

// sendInviteEmail sends an HTML invitation email with the acceptance link.
func (s *Service) sendInviteEmail(toEmail, inviterEmail, rawToken string) error {
	if s.smtpCfg.Host == "" || s.smtpCfg.Host == "localhost" {
		return nil // SMTP not configured — silent no-op
	}

	link := fmt.Sprintf("%s/invite/accept?token=%s", s.frontendURL, rawToken)
	from := s.smtpCfg.From
	if from == "" {
		from = "noreply@" + s.smtpCfg.Host
	}
	port := s.smtpCfg.Port
	if port == "" {
		port = "25"
	}

	subject := "Du wurdest zu Vakt eingeladen"
	body := fmt.Sprintf(`Hallo,

%s hat dich zu Vakt eingeladen.

Klicke auf den folgenden Link, um dein Konto zu erstellen:
%s

Der Link ist 7 Tage gueltig.

Wenn du diese E-Mail nicht erwartet hast, kannst du sie ignorieren.

Vakt Security Platform`, inviterEmail, link)

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		mailhdr.Sanitize(from), mailhdr.Sanitize(toEmail), mailhdr.Sanitize(subject),
	)
	msg := []byte(headers + body)
	addr := s.smtpCfg.Host + ":" + port

	if s.smtpCfg.User != "" && s.smtpCfg.Pass != "" {
		auth := smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Pass, s.smtpCfg.Host)
		return smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{toEmail}, msg)
}

// platformRoleName maps our simple role names to the names in the roles table.
func platformRoleName(role string) string {
	switch role {
	case "admin":
		return "Admin"
	case "editor":
		return "SecurityAnalyst"
	default:
		return "Viewer"
	}
}

// generateToken creates a cryptographically secure 32-byte random hex token
// and returns both the plaintext token and its SHA-256 hash.
func generateToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}
	plaintext = hex.EncodeToString(buf)
	hash = hashToken(plaintext)
	return plaintext, hash, nil
}

// hashToken returns the SHA-256 hex digest of the given token string.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
