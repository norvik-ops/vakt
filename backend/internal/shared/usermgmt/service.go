package usermgmt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/smtp"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/matharnica/vakt/internal/shared/apperr"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
	"github.com/matharnica/vakt/internal/shared/password"
)

// ErrSMTPNotConfigured meldet, dass die Einladungsmail NICHT hinausging, weil
// kein SMTP-Server eingerichtet ist.
//
// Bewusst ein eigener Sentinel statt notifications.ErrNotConfigured: usermgmt
// braucht von notifications sonst nichts, und eine Paket-Abhaengigkeit fuer
// einen Fehlerwert waere Kopplung ohne Gegenwert.
var ErrSMTPNotConfigured = errors.New("usermgmt: SMTP nicht eingerichtet — Einladung nicht versendet")

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
// Org membership is determined by org_members, so only users in the org are
// returned.
//
// Both role columns are reported: platform_role from org_members (authoritative,
// ADR-0077) and role from the users.role cache. A UI that shows only the cache
// cannot distinguish a Viewer from an InternalAuditor or an AuditorReadOnly —
// all three cache as "viewer" — so the team list would show the org's internal
// auditor as a plain viewer and offer no way to see who holds the SoD role.
func (s *Service) ListUsers(ctx context.Context, orgID string) ([]UserWithRole, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text, u.email, COALESCE(u.display_name, '') AS name,
		       u.role, r.name AS platform_role, u.created_at
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		JOIN roles r ON r.id = om.role_id
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
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PlatformRole, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if users == nil {
		users = []UserWithRole{}
	}
	return users, rows.Err()
}

// roleAssignment is one row of the role vocabulary: the authoritative platform
// role that goes into org_members, and the users.role cache value that must
// accompany it.
type roleAssignment struct {
	platform string // roles.name — the authoritative role (ADR-0077)
	simple   string // users.role — the denormalised cache; authorises nothing
}

// assignableRoles maps every value UpdateRoleInput.Role accepts to the pair of
// values a role change has to write. It is the single table for this boundary;
// platformRoleName (the invitation-accept path) is derived from it below, and
// admin.simpleUserRole is the same mapping at the INSERT boundary — keep them in
// sync, the mapping itself is fixed by ADR-0077.
//
// AuditorReadOnly and InternalAuditor map to the "viewer" cache value on purpose:
// the column has only three values (migration 077) and neither role is an admin.
// The cache deliberately loses information (ADR-0077 accepts that; it is a cache,
// not a source) — which is why UserWithRole also reports PlatformRole. Keeping it
// correct is still required at every boundary (scripts/check_user_role_insert.py),
// but since ESK-13 nothing authorises over it: requireAdmin, its last reader with
// a decision to make, reads org_members.
var assignableRoles = map[string]roleAssignment{
	"Admin":           {platform: "Admin", simple: "admin"},
	"SecurityAnalyst": {platform: "SecurityAnalyst", simple: "editor"},
	"Viewer":          {platform: "Viewer", simple: "viewer"},
	"AuditorReadOnly": {platform: "AuditorReadOnly", simple: "viewer"},
	"InternalAuditor": {platform: "InternalAuditor", simple: "viewer"},
	// Legacy aliases — the vocabulary this endpoint accepted before platform
	// roles became assignable. The frontend sent these; old clients still do.
	"admin":  {platform: "Admin", simple: "admin"},
	"editor": {platform: "SecurityAnalyst", simple: "editor"},
	"viewer": {platform: "Viewer", simple: "viewer"},
}

// THE UPDATE BOUNDARY, written down (REV-ESK13 §2.3).
//
// ADR-0077 and scripts/check_user_role_insert.py pin the INSERT boundary: every
// site that inserts an org_members row must set users.role to match. Nobody had
// written the rule for the UPDATE side, and the result was an endpoint that moved
// only the cache. The rule is:
//
//	org_members.role_id is the role. It goes into the token claim, every
//	auth.RequireRole guard reads it, and since ESK-13 usermgmt.requireAdmin and
//	ensureNotLastAdmin read it too. users.role is a lossy cache of it and
//	authorises nothing.
//
//	UpdateUserRole is the ONLY place that may change either column after the
//	insert, and it changes both in one transaction. A second UPDATE site that
//	writes just one of them re-creates the drift migration 253 had to clean up.
//
// This is a comment and not a CI gate on purpose: scripts/ is outside this
// change's file ownership. check_user_role_insert.py is the model to copy — the
// missing counterpart greps for UPDATE statements against org_members.role_id or
// users.role and allows this file as their only site.

// UpdateUserRole changes a user's role within the organisation. It prevents
// demoting the last remaining admin. On role change the user's active sessions
// are revoked so the new role takes effect at the next login.
//
// It writes BOTH role columns in one transaction, org_members first:
// org_members.role_id is what the login puts into the token claim and what every
// auth.RequireRole guard reads (ADR-0077), users.role is its cache. Before this,
// the endpoint wrote only the cache — so "make this member an Admin" left their
// claims at Viewer, and "demote this Admin to viewer" left them with full Admin
// claims on every module while the team list showed "Betrachter". Both directions
// were measured live against a real instance (ESK-13).
func (s *Service) UpdateUserRole(ctx context.Context, orgID, userID, role string) error {
	assign, ok := assignableRoles[role]
	if !ok {
		// Fail closed. The validator should have caught this; if the two lists
		// ever drift, an unknown role must be an error and not a silent
		// downgrade to Viewer (which would look like success in the UI).
		return fmt.Errorf("unknown role %q", role)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("update user role: begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Prevent removing the last admin — inside the transaction, so the rows it
	// counted stay locked until the change is committed (see ensureNotLastAdmin).
	if assign.platform != "Admin" {
		if err = s.ensureNotLastAdmin(ctx, tx, orgID, userID); err != nil {
			return err
		}
	}

	// Resolve the role id explicitly so a role that is missing from the roles
	// table (e.g. an instance that never ran migration 202, which seeds
	// InternalAuditor) fails with a readable error instead of a FK violation.
	var roleID string
	if err = tx.QueryRow(ctx,
		`SELECT id::text FROM roles WHERE name = $1`, assign.platform,
	).Scan(&roleID); err != nil {
		return fmt.Errorf("lookup role %q: %w", assign.platform, err)
	}

	// 1) The authoritative role. Scoped to the caller's org, so an admin can
	//    only re-role members of their own organisation.
	var result pgconn.CommandTag
	result, err = tx.Exec(ctx, `
		UPDATE org_members SET role_id = $1::uuid
		WHERE org_id = $2::uuid AND user_id = $3::uuid`,
		roleID, orgID, userID,
	)
	if err != nil {
		return fmt.Errorf("update org member role: %w", err)
	}
	if result.RowsAffected() == 0 {
		err = fmt.Errorf("user not found in organisation")
		return err
	}

	// 2) The cache. Same transaction — the two must never be observed apart.
	_, err = tx.Exec(ctx, `
		UPDATE users SET role = $1
		WHERE id = $2::uuid
		  AND EXISTS (
		    SELECT 1 FROM org_members WHERE org_id = $3::uuid AND user_id = $2::uuid
		  )`,
		assign.simple, userID, orgID,
	)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("update user role: commit: %w", err)
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("remove user: begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = s.ensureNotLastAdmin(ctx, tx, orgID, userID); err != nil {
		return err
	}

	var result pgconn.CommandTag
	result, err = tx.Exec(ctx, `
		DELETE FROM org_members
		WHERE org_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove user: %w", err)
	}
	if result.RowsAffected() == 0 {
		err = fmt.Errorf("user not found in organisation")
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("remove user: commit: %w", err)
	}
	if s.sessionRevoker != nil {
		if rErr := s.sessionRevoker.RevokeAllSessions(ctx, userID); rErr != nil {
			_ = rErr // best-effort; user is already removed from org_members
		}
	}
	return nil
}

// ErrUserNotInOrg is returned when a target user is not a member of the caller's org.
var ErrUserNotInOrg = errors.New("user not found in organisation")

// ResetUserMFA is the MFA break-glass (S131-R-H23): an admin clears a member's
// TOTP secret and recovery codes so a user who lost BOTH their authenticator AND
// their recovery codes can log in with their password and re-enrol. If the org
// requires MFA, MFAEnforceMiddleware forces re-enrolment on the (exempt)
// /auth/2fa/setup path. Org-scoped — an admin can only reset members of their own
// org (verified before any delete). The user's sessions are revoked (+ pw_version
// bumped via the revoker) so a still-logged-in user cannot skip the re-enrolment.
func (s *Service) ResetUserMFA(ctx context.Context, orgID, userID string) error {
	var member bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id = $1::uuid AND user_id = $2::uuid)`,
		orgID, userID).Scan(&member); err != nil {
		return fmt.Errorf("reset mfa: membership check: %w", err)
	}
	if !member {
		return ErrUserNotInOrg
	}
	// idempotent: break-glass reset is delete-if-exists — a user with no TOTP (0 rows) is success, not not-found.
	if _, err := s.db.Exec(ctx, `DELETE FROM totp_secrets WHERE user_id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("reset mfa: delete totp: %w", err)
	}
	// idempotent: same — recovery codes may already be absent; 0 rows is success.
	if _, err := s.db.Exec(ctx, `DELETE FROM auth_recovery_codes WHERE user_id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("reset mfa: delete recovery codes: %w", err)
	}
	if s.sessionRevoker != nil {
		_ = s.sessionRevoker.RevokeAllSessions(ctx, userID)
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

	// Store the platform role name, whichever vocabulary the caller used
	// (R1-14cA-11). One canonical value in the column means the accept path has
	// nothing to guess, and the legacy names stay readable for rows written
	// before migration 255.
	storedRole := assignmentFor(in.Role).platform

	var inv Invitation
	err = s.db.QueryRow(ctx, `
		INSERT INTO user_invitations (org_id, email, role, token_hash, invited_by)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, org_id::text, email, role, invited_by,
		          accepted_at, expires_at, created_at`,
		orgID, in.Email, storedRole, tokenHash, inviterEmail,
	).Scan(
		&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.InvitedBy,
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		return Invitation{}, fmt.Errorf("insert invitation: %w", err)
	}

	// Der Versand ist nicht toedlich: die Einladung steht in user_invitations,
	// ist in der Oberflaeche sichtbar und laeuft nach 7 Tagen ab. Anders als bei
	// den Fristen-Warnungen (R1-W9C-N1) wird hier also kein Zustand verbrannt.
	//
	// Verschluckt werden darf der Fehler trotzdem nicht: `_ = sendErr` hat jeden
	// echten Zustellfehler unsichtbar gemacht — der Administrator sah eine
	// angelegte Einladung und nahm an, die Mail sei unterwegs.
	//
	// Der Token wird BEWUSST NICHT geloggt. Der alte Kommentar an dieser Stelle
	// schlug genau das vor ("Log the token so the admin can share it manually") —
	// das waere ein Anmelde-Geheimnis im Klartext im Log. Der Administrator holt
	// den Link stattdessen aus der Einladungsliste.
	if sendErr := s.sendInviteEmail(in.Email, inviterEmail, rawToken); sendErr != nil {
		log.Warn().Err(sendErr).
			Str("to_redacted", logsafe.RedactEmail(in.Email)).
			Str("invitation_id", inv.ID).
			Msg("CreateInvitation: Einladungsmail nicht zugestellt — die Einladung " +
				"steht in der Liste und kann von dort geteilt werden")
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
		return fmt.Errorf("invitation %w", apperr.ErrNotFound)
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

	// Both role columns come from one lookup (R1-14cA-11). users.role carries
	// CHECK (role IN ('admin','editor','viewer')), and this used to insert the
	// invitation's raw string — which was safe only as long as the invitation
	// vocabulary happened to be exactly those three. The moment a platform name
	// like "Admin" can be invited, the raw write violates the constraint at
	// 'accept' time: after the invitee clicked the link and chose a password,
	// with no way for them to recover.
	assign := assignmentFor(role)

	// Create the user.
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`,
		email, string(passwordHash), in.Name, assign.simple,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	// Look up the matching platform role (Admin/SecurityAnalyst/Viewer) from the
	// roles table.
	platformRole := assign.platform
	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = $1`, platformRole).Scan(&roleID)
	if err != nil {
		// Fall back to Viewer if the role is not found.
		err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Viewer'`).Scan(&roleID)
		if err != nil {
			return fmt.Errorf("lookup role: %w", err)
		}
	}

	// Add org membership. MustAffect statt verworfenem CommandTag: org_members ist
	// seit ESK-13 die Quelle der Wahrheit fuer die Autorisierung. Ein INSERT, der
	// keine Zeile schreibt, wuerde hier eine Einladung als angenommen markieren,
	// ohne dass der Eingeladene je Rechte bekommt — genau die stille Null, die
	// dieser Lauf mehrfach gefunden hat.
	tag, err := tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID,
	)
	if err := shareddb.MustAffect(tag, err); err != nil {
		return fmt.Errorf("insert org member: %w", err)
	}

	// Mark invitation as accepted.
	// orgid-lint: global — UPDATE by PK; invID was verified via the org-scoped invite lookup earlier in this tx
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

// ErrLastAdmin is returned when a change would leave the organisation without an
// Admin. It is a named error so the handler can say WHICH rule refused, instead
// of the generic "Rolle konnte nicht aktualisiert werden" — an operator who is
// not told that the last-admin guard fired reaches for the database, which is the
// self-help ESK-13 exists to remove.
var ErrLastAdmin = errors.New("cannot remove or demote the last admin")

// ensureNotLastAdmin returns ErrLastAdmin if userID is the last Admin in orgID.
//
// It counts org_members — the authoritative role (ADR-0074/ADR-0077) — and so
// does usermgmt.requireAdmin since ESK-13. That the two read the SAME column is
// the whole correctness argument: this guard exists to keep at least one member
// able to pass that gate, and a guard that counts a different column than the
// gate admits is only accidentally right. It was measurably wrong on the drift
// every pre-ESK-13 role change produced (REV-ESK13 §2.1: a permitted demotion
// left an organisation with zero members who could still assign roles).
//
// It takes the transaction doing the change rather than the pool, and that is not
// cosmetic: FOR UPDATE OF om locks this org's Admin membership rows until that
// transaction commits, so two concurrent demotions serialise instead of both
// reading "2 admins" and both being let through — the TOCTOU the old pool-level
// guard had.
func (s *Service) ensureNotLastAdmin(ctx context.Context, tx pgx.Tx, orgID, userID string) error {
	rows, err := tx.Query(ctx, `
		SELECT om.user_id::text
		FROM org_members om
		JOIN roles r ON r.id = om.role_id
		WHERE om.org_id = $1::uuid AND r.name = 'Admin'
		FOR UPDATE OF om`,
		orgID,
	)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	defer rows.Close()

	adminCount := 0
	targetIsAdmin := false
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan admin: %w", err)
		}
		adminCount++
		if id == userID {
			targetIsAdmin = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}

	if targetIsAdmin && adminCount <= 1 {
		return ErrLastAdmin
	}
	if targetIsAdmin {
		return nil
	}

	// Not an admin — but still has to be a member of this org, which the old
	// version learned from the role lookup it no longer needs.
	var member bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM org_members WHERE org_id = $1::uuid AND user_id = $2::uuid)`,
		orgID, userID,
	).Scan(&member); err != nil {
		return fmt.Errorf("fetch target user role: %w", err)
	}
	if !member {
		return ErrUserNotInOrg
	}
	return nil
}

// sendInviteEmail sends an HTML invitation email with the acceptance link.
func (s *Service) sendInviteEmail(toEmail, inviterEmail, rawToken string) error {
	if s.smtpCfg.Host == "" || s.smtpCfg.Host == "localhost" {
		// Kein nil: "nicht gesendet" ist kein Erfolg. Der Aufrufer loggt es als
		// Warnung, nicht als Stoerfall — eine Instanz ohne SMTP ist zulaessig.
		return ErrSMTPNotConfigured
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

// assignmentFor maps an invitation's stored role to the pair of values account
// creation has to write — the authoritative org_members role and the users.role
// cache.
//
// Derived from assignableRoles so there is one table, not two. The fallback is
// deliberately different from UpdateUserRole's: an invitation row carries a value
// InviteInput validated when the invitation was created, possibly under an older
// vocabulary, and if an unknown one shows up here the safe answer for account
// creation is the least-privilege role — not an error that blocks the invitee
// from signing up at all.
func assignmentFor(role string) roleAssignment {
	if assign, ok := assignableRoles[role]; ok {
		return assign
	}
	return roleAssignment{platform: "Viewer", simple: "viewer"}
}

// platformRoleName maps an invitation's role to the name in the roles table.
func platformRoleName(role string) string {
	return assignmentFor(role).platform
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
