package auditor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service handles auditor invite and session business logic.
type Service struct {
	db *pgxpool.Pool
}

// NewService constructs an auditor Service.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// CreateInvite generates a new auditor invite token, stores its SHA-256 hash,
// and returns the invite record together with the plaintext token (shown once).
func (s *Service) CreateInvite(ctx context.Context, orgID, invitedByUserID string, in CreateInviteInput) (*AuditorInvite, string, error) {
	rawToken, tokenHash, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate invite token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(time.Duration(in.ExpiresIn) * 24 * time.Hour)

	var inv AuditorInvite
	err = s.db.QueryRow(ctx, `
		INSERT INTO auditor_invites (org_id, email, token_hash, invited_by, expires_at)
		VALUES ($1::uuid, $2, $3, $4::uuid, $5)
		RETURNING id::text, org_id::text, email, expires_at, accepted_at, created_at`,
		orgID, in.Email, tokenHash, invitedByUserID, expiresAt,
	).Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("insert auditor invite: %w", err)
	}

	return &inv, rawToken, nil
}

// AcceptInvite validates the invite token, marks it accepted, creates an active
// session, and returns the session token (plaintext, shown once).
func (s *Service) AcceptInvite(ctx context.Context, token string) (string, error) {
	tokenHash := hashToken(token)

	// Look up a valid (non-expired, non-accepted) invite by token hash.
	var inviteID, orgID, email string
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, email
		FROM auditor_invites
		WHERE token_hash = $1
		  AND accepted_at IS NULL
		  AND expires_at > NOW()`,
		tokenHash,
	).Scan(&inviteID, &orgID, &email)
	if err != nil {
		return "", fmt.Errorf("invite not found or expired")
	}

	// Mark invite as accepted.
	_, err = s.db.Exec(ctx, `
		UPDATE auditor_invites SET accepted_at = NOW() WHERE id = $1::uuid`,
		inviteID,
	)
	if err != nil {
		return "", fmt.Errorf("mark invite accepted: %w", err)
	}

	// Create session token — same TTL as the original invite expiry (use 30 days default).
	sessionRaw, sessionHash, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	sessionExpiry := time.Now().UTC().Add(30 * 24 * time.Hour)

	_, err = s.db.Exec(ctx, `
		INSERT INTO auditor_sessions (org_id, invite_id, token_hash, auditor_email, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		orgID, inviteID, sessionHash, email, sessionExpiry,
	)
	if err != nil {
		return "", fmt.Errorf("create auditor session: %w", err)
	}

	return sessionRaw, nil
}

// ValidateSession looks up a session by token hash and returns the auditor claims.
func (s *Service) ValidateSession(ctx context.Context, sessionToken string) (*AuditorClaims, error) {
	tokenHash := hashToken(sessionToken)

	var claims AuditorClaims
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, auditor_email, expires_at
		FROM auditor_sessions
		WHERE token_hash = $1
		  AND expires_at > NOW()`,
		tokenHash,
	).Scan(&claims.SessionID, &claims.OrgID, &claims.AuditorEmail, &claims.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("session not found or expired")
	}

	return &claims, nil
}

// ListInvites returns all auditor invites for the given organisation.
func (s *Service) ListInvites(ctx context.Context, orgID string) ([]AuditorInvite, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, email, expires_at, accepted_at, created_at
		FROM auditor_invites
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list auditor invites: %w", err)
	}
	defer rows.Close()

	var invites []AuditorInvite
	for rows.Next() {
		var inv AuditorInvite
		if err := rows.Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan auditor invite: %w", err)
		}
		invites = append(invites, inv)
	}
	if invites == nil {
		invites = []AuditorInvite{}
	}
	return invites, rows.Err()
}

// RevokeInvite deletes an auditor invite (and cascades to sessions) for the
// given organisation. Returns an error if the invite does not belong to orgID.
func (s *Service) RevokeInvite(ctx context.Context, orgID, id string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM auditor_invites
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete auditor invite: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("invite not found")
	}
	return nil
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
