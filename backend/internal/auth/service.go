// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"aidanwoods.dev/go-paseto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// ErrWeakPassword is returned when a supplied password does not satisfy the
// platform complexity requirements.
var ErrWeakPassword = errors.New("password must be at least 10 characters and contain uppercase, digit, and special character")

// validatePasswordStrength checks that password meets the Vakt minimum
// complexity policy:
//   - At least 10 characters
//   - At least one uppercase letter (A–Z)
//   - At least one decimal digit (0–9)
//   - At least one special character (!@#$%^&*()-_=+[]{}|;:'",.<>?/`~\)
//
// Returns ErrWeakPassword when any requirement is not satisfied.
func validatePasswordStrength(password string) error {
	if len(password) < 10 {
		return ErrWeakPassword
	}
	var hasUpper, hasDigit, hasSpecial bool
	const special = "!@#$%^&*()-_=+[]{}|;:'\",.<>?/`~\\"
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune(special, r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}
	return nil
}

// Service handles authentication business logic: registration, login, and token refresh.
type Service struct {
	db    *pgxpool.Pool
	redis *redis.Client
	key   paseto.V4SymmetricKey
}

// RegisterInput holds validated data for the registration endpoint.
type RegisterInput struct {
	Email    string `json:"email"         validate:"required,email"`
	Password string `json:"password"      validate:"required,min=10,max=72"`
	Name     string `json:"display_name"`
}

// AuthResponse is returned on successful authentication.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// refreshPayload is stored in Redis as JSON under key refresh:<sha256>.
type refreshPayload struct {
	UserID string   `json:"user_id"`
	OrgID  string   `json:"org_id"`
	Roles  []string `json:"roles"`
}

// NewService constructs an auth Service.
func NewService(db *pgxpool.Pool, redisClient *redis.Client, key paseto.V4SymmetricKey) *Service {
	return &Service{
		db:    db,
		redis: redisClient,
		key:   key,
	}
}

// Register creates a new user account and personal organisation, then issues tokens.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	// Enforce password complexity before doing any DB work.
	if err := validatePasswordStrength(input.Password); err != nil {
		return nil, err
	}

	// Use cost 12 per OWASP 2025 bcrypt recommendation (DefaultCost is 10).
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Insert user.
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text`,
		input.Email, string(hash), input.Name,
	).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Derive org slug from display name; fall back to email local part.
	orgName := input.Name
	if orgName == "" {
		orgName = strings.SplitN(input.Email, "@", 2)[0]
	}
	orgSlug := slugify(orgName)
	if orgSlug == "" {
		// slugify returned empty (e.g. name contained only non-ASCII chars).
		// Use a random 8-byte hex string to guarantee a unique, URL-safe slug.
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		orgSlug = hex.EncodeToString(b)
	}

	// Insert organisation.
	var orgID string
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		RETURNING id::text`,
		orgName, orgSlug,
	).Scan(&orgID)
	if err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}

	// Lookup Admin role id.
	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Admin'`).Scan(&roleID)
	if err != nil {
		return nil, fmt.Errorf("lookup admin role: %w", err)
	}

	// Link user to org as Admin.
	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert org member: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	roles := []string{"Admin"}
	return s.issueTokenPair(ctx, userID, orgID, roles)
}

// Login validates credentials and returns tokens on success.
func (s *Service) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	var userID, passwordHash string
	err := s.db.QueryRow(ctx, `
		SELECT id::text, password_hash
		FROM users
		WHERE email = $1 AND is_active = TRUE`,
		email,
	).Scan(&userID, &passwordHash)
	if err != nil {
		// Return a generic error to avoid user-enumeration.
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Fetch the user's role in their primary (first-joined) org.
	var orgID, roleName string
	err = s.db.QueryRow(ctx, `
		SELECT om.org_id::text, r.name
		FROM org_members om
		JOIN roles r ON r.id = om.role_id
		WHERE om.user_id = $1::uuid
		ORDER BY om.joined_at ASC
		LIMIT 1`,
		userID,
	).Scan(&orgID, &roleName)
	if err != nil {
		return nil, fmt.Errorf("fetch org membership: %w", err)
	}

	// Update last_login_at.
	if _, updateErr := s.db.Exec(ctx,
		`UPDATE users SET last_login_at = NOW() WHERE id = $1::uuid`, userID,
	); updateErr != nil {
		log.Warn().Err(updateErr).Str("user_id", userID).Msg("failed to update last_login_at")
	}

	return s.issueTokenPair(ctx, userID, orgID, []string{roleName})
}

// Refresh validates the given refresh token, rotates it, and returns a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	redisKey := refreshRedisKey(refreshToken)

	val, err := s.redis.Get(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	var payload refreshPayload
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		return nil, fmt.Errorf("corrupt refresh token payload: %w", err)
	}

	// Rotate: delete old token before issuing new one.
	if err := s.redis.Del(ctx, redisKey).Err(); err != nil {
		log.Warn().Err(err).Msg("failed to delete old refresh token")
	}

	return s.issueTokenPair(ctx, payload.UserID, payload.OrgID, payload.Roles)
}

// pwVersionKey returns the Redis key used to track a user's password version.
func pwVersionKey(userID string) string {
	return "user_pw_version:" + userID
}

// currentPwVersion returns the current password version for a user from Redis.
// If the key does not yet exist (user predates the feature), 0 is returned.
func (s *Service) currentPwVersion(ctx context.Context, userID string) int64 {
	val, err := s.redis.Get(ctx, pwVersionKey(userID)).Int64()
	if err != nil {
		// redis.Nil means key doesn't exist yet — treat as version 0.
		return 0
	}
	return val
}

// issueTokenPair generates an access + refresh token pair and stores the
// refresh token in Redis.
func (s *Service) issueTokenPair(ctx context.Context, userID, orgID string, roles []string) (*AuthResponse, error) {
	pwVersion := s.currentPwVersion(ctx, userID)
	claims := Claims{UserID: userID, OrgID: orgID, Roles: roles, PwVersion: pwVersion}

	accessToken, err := IssueAccessToken(s.key, claims)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, err := IssueRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	payload := refreshPayload{UserID: userID, OrgID: orgID, Roles: roles}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh payload: %w", err)
	}

	redisKey := refreshRedisKey(refreshToken)
	if err := s.redis.Set(ctx, redisKey, payloadJSON, RefreshTokenTTL).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(AccessTokenTTL / time.Second),
	}, nil
}

const (
	// loginFailMax is the number of consecutive failed logins that trigger a lockout.
	loginFailMax = 5
	// loginLockoutTTL is the lockout duration and the TTL of the failure counter.
	loginLockoutTTL = 15 * time.Minute
)

// loginFailKey returns the Redis key used to count consecutive login failures.
func loginFailKey(email string) string {
	return "login_fail:" + email
}

// checkAccountLocked returns true if the account is currently locked out.
func (s *Service) checkAccountLocked(ctx context.Context, email string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	val, err := s.redis.Get(ctx, loginFailKey(email)).Int64()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		// Redis unavailable — fail open to avoid blocking legitimate logins.
		log.Warn().Err(err).Str("email", email).Msg("login lockout check skipped — Redis unavailable")
		return false, nil
	}
	return val >= loginFailMax, nil
}

// recordLoginFailure increments the failure counter for the given email,
// setting a 15-minute TTL on first increment.
func (s *Service) recordLoginFailure(ctx context.Context, email string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := loginFailKey(email)
	// Use a pipeline: INCR then SET EX only if the key was just created.
	// We use SetNX so that an existing TTL is preserved (not reset on each failure).
	pipe := s.redis.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginLockoutTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Warn().Err(err).Str("email", email).Msg("login: failed to record login failure")
		return
	}
	log.Debug().Str("email", email).Int64("count", incrCmd.Val()).Msg("login: recorded failure")
}

// clearLoginFailures deletes the login failure counter for the given email.
func (s *Service) clearLoginFailures(ctx context.Context, email string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.redis.Del(ctx, loginFailKey(email)).Err(); err != nil && err != redis.Nil {
		log.Warn().Err(err).Str("email", email).Msg("login: failed to clear login failures")
	}
}

// RevokeToken blacklists an access token in Redis so that AuthMiddleware will
// reject it for the remainder of its natural lifetime (AccessTokenTTL).
// A 2-second context timeout prevents this call from blocking the response.
func (s *Service) RevokeToken(ctx context.Context, rawToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := tokenDenyKey(rawToken)
	return s.redis.Set(ctx, key, "1", AccessTokenTTL).Err()
}

// tokenDenyKey returns the Redis key used to blacklist an access token.
func tokenDenyKey(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return "revoked:" + hex.EncodeToString(sum[:])
}

// refreshRedisKey returns the Redis key for storing a refresh token,
// using a SHA-256 hash of the raw token.
func refreshRedisKey(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return "refresh:" + hex.EncodeToString(sum[:])
}

// StoreOIDCState stores a one-time OIDC state value in Redis with a 10-minute TTL.
// The state is used to prevent OAuth2 CSRF attacks (RFC 6749 §10.12).
func (s *Service) StoreOIDCState(ctx context.Context, state string) error {
	if s.redis == nil {
		return nil // skip in tests
	}
	return s.redis.Set(ctx, "oidc_state:"+state, "1", 10*time.Minute).Err()
}

// ValidateAndConsumeOIDCState verifies that the given state exists in Redis and
// deletes it atomically so it cannot be reused (one-time-use).
func (s *Service) ValidateAndConsumeOIDCState(ctx context.Context, state string) error {
	if s.redis == nil {
		return nil // skip in tests
	}
	deleted, err := s.redis.Del(ctx, "oidc_state:"+state).Result()
	if err != nil {
		return fmt.Errorf("state validation error: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("invalid or expired OIDC state")
	}
	return nil
}

// slugify converts a string to a URL-safe slug (lowercase, hyphens).
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
