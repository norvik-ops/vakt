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
	"github.com/matharnica/vakt/internal/shared/logsafe"
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
	db       *pgxpool.Pool
	redis    *redis.Client
	key      paseto.V4SymmetricKey
	denyFall *denyListFallback // PostgreSQL fallback when Redis is unavailable
	// failOpenOnRedisOutage controls how lockout checks behave when Redis is
	// unreachable. Default false (= fail closed): if we cannot read the
	// counter we treat the request as locked and return a 503-shaped error
	// to the handler, denying the login attempt. This closes the brute-force
	// window that the audit flagged. Operators who would rather accept the
	// risk to stay available during a Redis outage can flip the flag by
	// setting VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true and calling
	// svc.WithFailOpenOnRedisOutage(true).
	failOpenOnRedisOutage bool
	// ipLockoutMax is the secondary pure-IP lockout threshold (across any email).
	// Configurable via WithIPLockoutMax; defaults to ipLockoutSecondaryFailMax.
	ipLockoutMax int
	// dummyBcryptHash is a precomputed bcrypt hash (cost 12) of a random value.
	// Login() compares against it when the e-mail is unknown so the bcrypt work
	// is constant-time regardless of whether the user exists — closing the
	// timing side-channel that allowed user enumeration (S87-3, F-05, CWE-208).
	dummyBcryptHash []byte
}

// WithFailOpenOnRedisOutage flips the lockout-check behaviour to "fail
// open" — i.e., let requests through when Redis is unreachable. Use only
// when the deployment explicitly accepts the brute-force-during-outage
// trade-off in favour of availability. See ADR-0044.
func (s *Service) WithFailOpenOnRedisOutage(b bool) *Service {
	s.failOpenOnRedisOutage = b
	return s
}

// WithIPLockoutMax sets the secondary pure-IP lockout threshold (across any email).
// Default is 50. Lower values block aggressive spraying faster; higher values
// are safer on shared NAT (corporate VPN, school networks) where many users
// share one IP.
func (s *Service) WithIPLockoutMax(n int) *Service {
	s.ipLockoutMax = n
	return s
}

// ErrLockoutCheckUnavailable is returned by the lockout helpers when Redis
// is unreachable AND the service is configured to fail closed. The login
// handler maps this to HTTP 503 to surface the dependency outage to the
// caller instead of letting them brute-force during it.
var ErrLockoutCheckUnavailable = errors.New("auth: lockout check unavailable (redis outage, fail-closed)")

// RegisterInput holds validated data for the registration endpoint.
type RegisterInput struct {
	Email    string `json:"email"         validate:"required,email"`
	Password string `json:"password"      validate:"required,min=10,max=72"`
	Name     string `json:"display_name"`
}

// AuthResponse is returned on successful authentication.
//
// Das Frontend (Login.tsx → useAuthStore.setAuth) erwartet das User-Objekt,
// um es im Zustand abzulegen und Rolle/Anzeigenamen darzustellen — fehlt es,
// crasht das Login mit "can't access property id".
type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"` // seconds
	User         AuthUser `json:"user"`
	// SessionID ist die UUID der refresh_sessions-Row. Frontend speichert sie,
	// damit die SessionsPage die "diese hier"-Session markieren kann.
	SessionID string `json:"session_id,omitempty"`
	// CSRFToken spiegelt den csrf_token-Cookie-Wert im Response-Body.
	// Grund: Reverse Proxies/CDNs vor der Instanz können Set-Cookie-Header
	// umschreiben (z.B. HttpOnly nachträglich setzen), wodurch das Frontend
	// den Cookie nicht mehr per document.cookie lesen kann, obwohl der Browser
	// ihn weiterhin korrekt mitsendet. Der Body-Wert ist von sowas unberührt
	// und dient dem Frontend als zuverlässiger Fallback (client.ts).
	CSRFToken string `json:"csrf_token,omitempty"`
}

// AuthUser ist die minimal nötige User-Repräsentation für Frontend-State.
type AuthUser struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
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
		db:              db,
		redis:           redisClient,
		key:             key,
		denyFall:        &denyListFallback{db: db},
		dummyBcryptHash: newDummyBcryptHash(),
		ipLockoutMax:    ipLockoutSecondaryFailMax,
	}
}

// newDummyBcryptHash precomputes a bcrypt hash (cost 12) of a cryptographically
// random value. It is compared against during failed logins for unknown e-mails
// so the bcrypt cost is paid on every attempt, eliminating the timing oracle.
func newDummyBcryptHash() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand failure is fatal-grade, but Login must still leave constant
		// work; fall back to a fixed value so CompareHashAndPassword still runs.
		secret = []byte("vakt-dummy-timing-defense-fallback")
	}
	h, err := bcrypt.GenerateFromPassword(secret, 12)
	if err != nil {
		// Should never happen; return a static valid-cost-12 hash so the compare
		// path stays non-nil. ("invalid" placeholder is never the right password.)
		return []byte("$2a$12$abcdefghijklmnopqrstuuJ9z7yXqj8c.0xZ3o9kF1m2n3o4p5q6r")
	}
	return h
}

// ErrRegistrationDisabled is returned when a registration attempt is made
// after the initial setup org has already been created.
var ErrRegistrationDisabled = errors.New("registration is disabled — this instance is already set up")

// Register creates a new user account and personal organisation, then issues tokens.
// deviceHint is the caller's User-Agent header (truncated to 120 chars) used for
// per-device session tracking; pass "" when not available.
//
// Registration is only allowed when no organisation exists yet (bootstrap).
// Once the first org is created, this endpoint returns ErrRegistrationDisabled
// so that publicly reachable instances cannot be used to create arbitrary tenants.
func (s *Service) Register(ctx context.Context, input RegisterInput, deviceHint string) (*AuthResponse, error) {
	// Guard: allow only the very first registration (bootstrap).
	// Demo orgs created via RunEphemeral() bypass this path entirely.
	var orgCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM organizations`).Scan(&orgCount); err != nil {
		return nil, fmt.Errorf("registration check: %w", err)
	}
	if orgCount > 0 {
		return nil, ErrRegistrationDisabled
	}

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
	resp, tokErr := s.issueTokenPair(ctx, userID, orgID, roles, deviceHint)
	if tokErr == nil {
		// S22-3: Register-Flow = erfolgreicher Erst-Login.
		s.recordLogin(ctx, orgID, userID, input.Email, deviceHint, "register", "ok")
	}
	return resp, tokErr
}

// Login validates credentials and returns tokens on success.
// deviceHint is the caller's User-Agent header (truncated to 120 chars).
//
// Sprint 20 / S20-6: pro Versuch (Erfolg oder Fehlschlag) ein Eintrag in
// login_history. Best-Effort, Fehler blockieren den Login nie.
func (s *Service) Login(ctx context.Context, email, password, deviceHint string) (*AuthResponse, error) {
	var userID, passwordHash, displayName string
	err := s.db.QueryRow(ctx, `
		SELECT id::text, password_hash, COALESCE(display_name, email)
		FROM users
		WHERE email = $1 AND is_active = TRUE`,
		email,
	).Scan(&userID, &passwordHash, &displayName)

	// S87-3 (F-05, CWE-208): always run bcrypt.CompareHashAndPassword, even for
	// unknown e-mails, so the response latency does not reveal whether the user
	// exists. On a DB miss we compare against a precomputed dummy hash. The error
	// text + code are identical in both failure branches (no enumeration signal).
	if err != nil {
		hashToCheck := s.dummyBcryptHash
		if len(hashToCheck) == 0 {
			hashToCheck = newDummyBcryptHash()
		}
		_ = bcrypt.CompareHashAndPassword(hashToCheck, []byte(password))
		// Failed-attempt-Record: kein user_id, nur email + IP-Spur.
		s.recordLogin(ctx, "", "", email, deviceHint, "password", "bad_password")
		// Return a generic error to avoid user-enumeration.
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		s.recordLogin(ctx, "", userID, email, deviceHint, "password", "bad_password")
		return nil, fmt.Errorf("invalid credentials")
	}

	// S13-6 Cost-Upgrade-on-Login: ueberprueft den Cost-Faktor des gespeicherten
	// Hashes. Wenn er unter dem aktuellen Pflicht-Cost (12) liegt, re-hashen wir
	// das gerade verifizierte Klartext-Passwort und schreiben den neuen Hash
	// zurueck. So bekommen Legacy-User (z.B. aus Tests mit MinCost) ohne
	// Passwort-Reset einen aktuellen Hash. Fehler beim Re-Hash brechen den
	// Login NICHT ab — nur Warn-Log, der Cost-Upgrade-Versuch ist Best-Effort.
	if cost, costErr := bcrypt.Cost([]byte(passwordHash)); costErr == nil && cost < 12 {
		if newHash, rehashErr := bcrypt.GenerateFromPassword([]byte(password), 12); rehashErr == nil {
			if _, updErr := s.db.Exec(ctx,
				`UPDATE users SET password_hash = $1 WHERE id = $2::uuid`,
				string(newHash), userID,
			); updErr != nil {
				log.Warn().Err(updErr).Str("user_id", userID).
					Int("old_cost", cost).Msg("bcrypt cost-upgrade: DB update failed")
			} else {
				log.Info().Str("user_id", userID).
					Int("old_cost", cost).Int("new_cost", 12).
					Msg("bcrypt cost-upgrade applied on login")
			}
		} else {
			log.Warn().Err(rehashErr).Str("user_id", userID).
				Msg("bcrypt cost-upgrade: GenerateFromPassword failed")
		}
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
	//
	// S90-8 (#8): this and the success-path recordLogin below run synchronously
	// and on purpose. Login is rate-limited (10/min per IP) and not a latency-hot
	// path, so the two tiny best-effort writes add no meaningful tail latency.
	// Keeping them synchronous avoids a context.WithoutCancel detach that could
	// race with pool teardown in tests and would obscure write failures — the
	// marginal speedup is not worth that complexity.
	if _, updateErr := s.db.Exec(ctx,
		`UPDATE users SET last_login_at = NOW() WHERE id = $1::uuid`, userID,
	); updateErr != nil {
		log.Warn().Err(updateErr).Str("user_id", userID).Msg("failed to update last_login_at")
	}

	resp, err := s.issueTokenPair(ctx, userID, orgID, []string{roleName}, deviceHint)
	if err != nil {
		return nil, err
	}
	resp.User = AuthUser{
		ID:          userID,
		Email:       email,
		DisplayName: displayName,
		Roles:       []string{roleName},
	}
	// Sprint 20 S20-6: Erfolgreichen Login persistieren.
	s.recordLogin(ctx, orgID, userID, email, deviceHint, "password", "ok")
	return resp, nil
}

// recordLogin schreibt einen login_history-Eintrag. Best-Effort — Fehler
// werden nicht propagiert, weil sie den Login-Pfad nicht blockieren dürfen.
// Sprint 20 S20-6.
func (s *Service) recordLogin(ctx context.Context, orgID, userID, email, userAgent, source, result string) {
	_, _ = s.db.Exec(ctx, `
		INSERT INTO login_history (org_id, user_id, email, user_agent, source, result)
		VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, NULLIF($3, ''),
		        NULLIF($4, ''), $5, $6)`,
		orgID, userID, email, userAgent, source, result,
	)
}

// Refresh validates the given refresh token, rotates it, and returns a new token pair.
// Roles are loaded fresh from the DB on every refresh so that demotions and removals
// take effect at the next token rotation (AUTH-007).
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

	// Verify the user is still active and belongs to the org; load role fresh from DB.
	var isActive bool
	var roleName string
	err = s.db.QueryRow(ctx, `
		SELECT u.is_active, r.name
		FROM users u
		JOIN org_members om ON om.user_id = u.id
		JOIN roles r ON r.id = om.role_id
		WHERE u.id = $1::uuid AND om.org_id = $2::uuid`,
		payload.UserID, payload.OrgID,
	).Scan(&isActive, &roleName)
	if err != nil {
		// User removed from org or deleted — invalidate token.
		return nil, fmt.Errorf("invalid or expired refresh token")
	}
	if !isActive {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	// Look up device hint from the session row so it carries forward to the new token.
	oldHash := sha256Hex(refreshToken)
	var deviceHint string
	_ = s.db.QueryRow(ctx,
		`SELECT device_hint FROM refresh_sessions WHERE token_hash = $1`, oldHash,
	).Scan(&deviceHint)

	// Rotate: delete old token before issuing new one.
	if err := s.redis.Del(ctx, redisKey).Err(); err != nil {
		log.Warn().Err(err).Msg("failed to delete old refresh token")
	}
	// Remove old session row; the new one will be inserted by issueTokenPair.
	_, _ = s.db.Exec(ctx, `DELETE FROM refresh_sessions WHERE token_hash = $1`, oldHash)

	return s.issueTokenPair(ctx, payload.UserID, payload.OrgID, []string{roleName}, deviceHint)
}

// pwVersionKey returns the Redis key used to track a user's password version.
func pwVersionKey(userID string) string {
	return "user_pw_version:" + userID
}

// currentPwVersion returns the current password version for a user from Redis.
// If the key does not yet exist (user predates the feature), 0 is returned.
// If Redis is not wired (integration tests that pass nil for the client),
// also fall back to 0 — the password-version invalidation is best-effort
// anyway, not a correctness guarantee.
func (s *Service) currentPwVersion(ctx context.Context, userID string) int64 {
	if s.redis == nil {
		return 0
	}
	val, err := s.redis.Get(ctx, pwVersionKey(userID)).Int64()
	if err != nil {
		// redis.Nil means key doesn't exist yet — treat as version 0.
		return 0
	}
	return val
}

// sha256Hex returns the hex-encoded SHA-256 hash of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// issueTokenPair generates an access + refresh token pair, stores the refresh
// token in Redis, and records the session in refresh_sessions for per-device
// revocation. deviceHint should be the User-Agent header truncated to 120 chars.
func (s *Service) issueTokenPair(ctx context.Context, userID, orgID string, roles []string, deviceHint string) (*AuthResponse, error) {
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

	if s.redis != nil {
		redisKey := refreshRedisKey(refreshToken)
		if err := s.redis.Set(ctx, redisKey, payloadJSON, RefreshTokenTTL).Err(); err != nil {
			return nil, fmt.Errorf("store refresh token: %w", err)
		}
	}

	// Persist session row for per-device listing and revocation.
	tokenHash := sha256Hex(refreshToken)
	expiresAt := time.Now().Add(RefreshTokenTTL)
	if len(deviceHint) > 120 {
		deviceHint = deviceHint[:120]
	}
	var sessionID string
	dbErr := s.db.QueryRow(ctx, `
		INSERT INTO refresh_sessions (user_id, org_id, token_hash, device_hint, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		ON CONFLICT (token_hash) DO UPDATE SET last_used = NOW()
		RETURNING id::text`,
		userID, orgID, tokenHash, deviceHint, expiresAt,
	).Scan(&sessionID)
	if dbErr != nil {
		// Non-fatal: Redis is the source of truth for token validity.
		log.Warn().Err(dbErr).Str("user_id", userID).Msg("issueTokenPair: failed to persist refresh session")
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(AccessTokenTTL / time.Second),
		SessionID:    sessionID,
	}, nil
}

const (
	// S121-F4 (F1-Auth): a pure per-email lockout (loginFailMax = 5, keyed only on
	// the address) used to live here. Because it ignored the source IP, ANY caller
	// could lock ANY account out of the product with five wrong passwords — a
	// trivial targeted denial of service, and never part of the documented design.
	// It has been removed; the two lockouts below are the whole scheme
	// (S107 / ADR-0044) and neither lets an attacker lock out a third party.

	// ipEmailLockoutFailMax is the number of failed login attempts for a specific
	// (IP, email) pair that trigger a per-pair lockout. Primary protection: stops
	// a single attacker from brute-forcing one account without affecting other
	// users behind the same NAT.
	ipEmailLockoutFailMax = 10
	// ipLockoutSecondaryFailMax is the secondary pure-IP lockout threshold
	// (across any email). Blocks broad credential-spraying attacks. Configurable
	// via WithIPLockoutMax / VAKT_RATELIMIT_IP_MAX; default 50.
	ipLockoutSecondaryFailMax = 50
	// ipLockoutTTL is the lockout duration for both IP-level lockouts.
	ipLockoutTTL = 15 * time.Minute
)

// loginIPFailKey returns the Redis key for counting per-IP login failures.
func loginIPFailKey(ip string) string {
	return "login_fail_ip:" + ip
}

// loginIPEmailFailKey returns the Redis key for counting per-(IP, email) failures.
// Using email directly is acceptable here — same exposure level as loginFailKey.
func loginIPEmailFailKey(ip, email string) string {
	return "login_fail_ip_email:" + ip + ":" + email
}

// checkIPEmailLocked returns true if the (IP, email) pair has exceeded the
// per-pair failure threshold. This is the primary NAT-safe lockout: only the
// specific account being targeted gets blocked, not the entire IP.
func (s *Service) checkIPEmailLocked(ctx context.Context, ip, email string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	val, err := s.redis.Get(ctx, loginIPEmailFailKey(ip, email)).Int64()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		log.Warn().Err(err).Str("ip", ip).Bool("fail_open", s.failOpenOnRedisOutage).Msg("ip+email lockout check: Redis unavailable")
		if s.failOpenOnRedisOutage {
			return false, nil
		}
		return true, ErrLockoutCheckUnavailable
	}
	return val >= ipEmailLockoutFailMax, nil
}

// recordIPEmailLoginFailure increments the per-(IP, email) failure counter.
func (s *Service) recordIPEmailLoginFailure(ctx context.Context, ip, email string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := loginIPEmailFailKey(ip, email)
	pipe := s.redis.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ipLockoutTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Warn().Err(err).Str("ip", ip).Msg("login: failed to record IP+email login failure")
		return
	}
	log.Debug().Str("ip", ip).Str("email_redacted", logsafe.RedactEmail(email)).Int64("count", incrCmd.Val()).Msg("login: recorded IP+email failure")
}

// clearLoginFailures deletes this user's (IP, email) failure counter after a
// successful login, so their own typos don't count against them next time.
//
// S121-F4: this used to clear the pure per-email counter, which meant the
// (IP, email) counter survived a successful login — a user who mistyped a few
// times and then signed in could still be locked out by a single further typo.
// The per-IP counter is deliberately left alone (see the call site).
func (s *Service) clearLoginFailures(ctx context.Context, ip, email string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.redis.Del(ctx, loginIPEmailFailKey(ip, email)).Err(); err != nil && err != redis.Nil {
		log.Warn().Err(err).Str("ip", ip).Str("email_redacted", logsafe.RedactEmail(email)).Msg("login: failed to clear login failures")
	}
}

// checkIPLocked returns true if the originating IP has exceeded the per-IP
// failure threshold, regardless of which email address was targeted.
func (s *Service) checkIPLocked(ctx context.Context, ip string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	val, err := s.redis.Get(ctx, loginIPFailKey(ip)).Int64()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		log.Warn().Err(err).Str("ip", ip).Bool("fail_open", s.failOpenOnRedisOutage).Msg("ip lockout check: Redis unavailable")
		if s.failOpenOnRedisOutage {
			return false, nil
		}
		return true, ErrLockoutCheckUnavailable
	}
	return val >= int64(s.ipLockoutMax), nil
}

// recordIPLoginFailure increments the per-IP failure counter.
func (s *Service) recordIPLoginFailure(ctx context.Context, ip string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := loginIPFailKey(ip)
	// ExpireNX: set TTL only on the first increment so the lockout window is
	// anchored to the first failure, not extended by each subsequent attempt.
	pipe := s.redis.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ipLockoutTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Warn().Err(err).Str("ip", ip).Msg("login: failed to record IP login failure")
		return
	}
	log.Debug().Str("ip", ip).Int64("count", incrCmd.Val()).Msg("login: recorded IP failure")
}

// RevokeToken blacklists an access token in Redis so that AuthMiddleware will
// reject it for the remainder of its natural lifetime (AccessTokenTTL).
// If Redis is unavailable, the token hash is written to the PostgreSQL fallback
// table (token_deny_list_fallback) so that revocation survives a Redis outage.
func (s *Service) RevokeToken(ctx context.Context, rawToken string) error {
	rCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := tokenDenyKey(rawToken)
	if err := s.redis.Set(rCtx, key, "1", AccessTokenTTL).Err(); err != nil {
		log.Warn().Err(err).Msg("RevokeToken: Redis unavailable — writing to PG fallback")
		s.denyFall.revokeInFallback(ctx, key, time.Now().Add(AccessTokenTTL))
		return nil
	}
	return nil
}

// RevokeAllSessions deletes all refresh sessions for the user from both the
// refresh_sessions table and the corresponding Redis keys, ensuring that a
// stolen refresh token cannot be used after the user logs out (AUTH-001).
func (s *Service) RevokeAllSessions(ctx context.Context, userID string) error {
	if s.db == nil {
		return fmt.Errorf("revoke sessions: db not available")
	}
	rows, err := s.db.Query(ctx,
		`DELETE FROM refresh_sessions WHERE user_id = $1::uuid RETURNING token_hash`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var h string
		if scanErr := rows.Scan(&h); scanErr == nil {
			keys = append(keys, "refresh:"+h)
		}
	}

	if s.redis != nil && len(keys) > 0 {
		rCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_ = s.redis.Del(rCtx, keys...) // best-effort
	}
	return nil
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
