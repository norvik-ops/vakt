package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/config"
	"github.com/rs/zerolog/log"
)

// OIDCCallbackInput is sent by the frontend after Casdoor redirects back.
type OIDCCallbackInput struct {
	Code     string `json:"code"     validate:"required"`
	State    string `json:"state"    validate:"required"`
	Provider string `json:"provider" validate:"required,oneof=google github keycloak"`
}

// OIDCProviders is the set of providers an OIDC login can be completed with.
//
// R1-07-B07-3: /auth/oidc/initiate took ANY provider value while the callback
// only knows these three. A user was redirected to the identity provider, signed
// in there successfully, and only THEN hit 422 — the failure landed at the
// latest possible moment, after authenticating at a foreign system, where
// nothing they can do fixes it.
//
// The value must stay in step with the oneof list in the Provider tag above.
// A struct tag cannot reference a variable, so the two are pinned together by
// oidc_provider_parity_test.go rather than by a comment.
var OIDCProviders = []string{"google", "github", "keycloak"}

// isSupportedOIDCProvider reports whether an OIDC login can be completed with
// this provider.
func isSupportedOIDCProvider(provider string) bool {
	for _, p := range OIDCProviders {
		if p == provider {
			return true
		}
	}
	return false
}

// SAMLCallbackInput carries the SAML response from the IdP.
type SAMLCallbackInput struct {
	SAMLResponse string `json:"saml_response" validate:"required"`
	RelayState   string `json:"relay_state"`
}

// ErrCasdoorNotConfigured is returned when CASDOOR_URL is not set.
var ErrCasdoorNotConfigured = errors.New("OIDC: configure CASDOOR_URL env var")

// ErrEmailNotVerified is returned when an OIDC provider hands back an email
// that has NOT been verified by the upstream IdP and a local user with that
// email already exists. Linking would let an attacker take over the local
// account by registering with the victim's email at any unverified-email-
// accepting IdP. See ADR-0033.
var ErrEmailNotVerified = errors.New("OIDC: email not verified by identity provider; refusing to link to existing account")

// ErrSAMLUserNotProvisioned is returned when JIT provisioning is disabled and
// the SAML user has no pre-existing account.
var ErrSAMLUserNotProvisioned = errors.New("SAML: user not found and JIT provisioning is disabled")

// ErrOIDCNoEmail is returned when the identity provider hands back a profile
// without an email address.
//
// The login has to fail here, and fail loudly. users.email is UNIQUE and NOT
// NULL, so provisioning such a profile stores an empty email — and that row
// then collides with every further email-less login (SQLSTATE 23505), turning a
// mapping error at ONE login into a permanent lock-out for all of them. That is
// the shape R1-07-B07-1 took live.
var ErrOIDCNoEmail = errors.New("OIDC: identity provider returned no email address")

// ErrNoOrganization is returned when an SSO login would have to provision a user
// but the instance carries no organisation yet.
//
// Vakt runs one customer per server and the organisation is founded once, by
// first-run setup. An SSO login is not a founding act — Service.Register refuses
// for the same reason once an organisation exists (ErrRegistrationDisabled).
var ErrNoOrganization = errors.New("SSO: instance has no organisation yet — run first-run setup before signing in via SSO")

// casdoorTokenResponse is the JSON response from Casdoor's token endpoint.
type casdoorTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

// casdoorUserProfile is the JSON response from Casdoor's get-account endpoint.
//
// EmailVerified maps Casdoor's `emailVerified` field. If the field is missing
// from the response the zero-value (false) is used — we treat that as
// "unverified" and refuse to link the OIDC subject to an existing local
// account. See ADR-0033 (OIDC email-verification gate).
type casdoorUserProfile struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	Avatar        string `json:"avatar"`
	Provider      string `json:"provider"`
}

// casdoorSAMLResponse is the JSON response from Casdoor's SAML login endpoint.
type casdoorSAMLResponse struct {
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Error string `json:"error"`
}

// casdoorEnvelope is the response Casdoor's own API controllers return:
// status/msg/sub/name at the top level, the account object nested under data.
//
// R1-07-B07-1: /api/get-account answers in exactly this shape, and email plus
// emailVerified exist ONLY inside data. Reading the body with a flat struct
// picked up sub (top level, so the "empty profile" guard stayed silent) and left
// email empty. Measured live against a real Casdoor.
//
// Data is deliberately a RawMessage: the field is absent from a plain OIDC
// userinfo body and carries something other than an account object on Casdoor's
// error responses, and neither may make the parse fail.
type casdoorEnvelope struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Data   json.RawMessage `json:"data"`
}

// casdoorAccount is the subset of Casdoor's user object this code consumes, as
// it appears inside the envelope's data field.
type casdoorAccount struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
	Avatar        string `json:"avatar"`
}

// parseCasdoorProfile reads a profile out of a Casdoor response body, whether it
// arrives wrapped in the controller envelope (/api/get-account) or flat (an
// OIDC-standard userinfo body). Flat values win; the nested account fills in
// what the top level does not carry — which for get-account is everything except
// sub and name.
//
// It stays tolerant of both shapes on purpose rather than switching to
// /api/userinfo: get-account returns the full account regardless of which scopes
// the OAuth client was granted, and the same function then also serves a
// non-Casdoor provider answering in the flat OIDC shape.
func parseCasdoorProfile(body []byte) (casdoorUserProfile, error) {
	var profile casdoorUserProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return casdoorUserProfile{}, err
	}

	var env casdoorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return casdoorUserProfile{}, err
	}
	if env.Status == "error" {
		msg := env.Msg
		if msg == "" {
			msg = "unknown error"
		}
		return casdoorUserProfile{}, fmt.Errorf("Casdoor: %s", msg)
	}
	if len(env.Data) == 0 {
		return profile, nil
	}

	var account casdoorAccount
	if err := json.Unmarshal(env.Data, &account); err != nil {
		// data carries something that is not an account object (Casdoor puts
		// varying payloads there). The flat read above is then all there is.
		return profile, nil //nolint:nilerr // a non-account data field is not a parse failure
	}

	if profile.Sub == "" {
		profile.Sub = account.ID
	}
	if profile.Email == "" {
		profile.Email = account.Email
	}
	if !profile.EmailVerified {
		profile.EmailVerified = account.EmailVerified
	}
	if profile.Avatar == "" {
		profile.Avatar = account.Avatar
	}
	// The envelope's top-level name is the account's user name; data.displayName
	// is the human-readable one. Prefer the latter for the profile's Name, which
	// becomes display_name and the org name at JIT provisioning.
	if account.DisplayName != "" {
		profile.Name = account.DisplayName
	} else if profile.Name == "" {
		profile.Name = account.Name
	}
	return profile, nil
}

// OIDCLogin exchanges the provider code for a Paseto token pair via Casdoor.
// When CasdoorURL is not configured the call returns ErrCasdoorNotConfigured
// so the frontend can display a proper error state.
// deviceHint is the caller's User-Agent header (truncated to 120 chars).
func (s *Service) OIDCLogin(ctx context.Context, cfg *config.Config, provider, code, state, deviceHint string) (*AuthResponse, error) {
	if cfg.CasdoorURL == "" {
		return nil, ErrCasdoorNotConfigured
	}

	// Step 1: Exchange authorization code for access token.
	redirectURI := strings.TrimRight(cfg.FrontendURL, "/") + "/auth/callback"
	tokenBody := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     cfg.CasdoorClientID,
		"client_secret": cfg.CasdoorClientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	}
	tokenBodyJSON, err := json.Marshal(tokenBody)
	if err != nil {
		return nil, fmt.Errorf("OIDC: marshal token request: %w", err)
	}

	tokenURL := strings.TrimRight(cfg.CasdoorURL, "/") + "/api/login/oauth/access_token"
	httpClient := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(tokenBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("OIDC: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC: token exchange request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("OIDC: read token response: %w", err)
	}

	var tokenResp casdoorTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("OIDC: parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OIDC: token exchange error: %s", tokenResp.Error)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("OIDC: empty access token from Casdoor")
	}

	// Step 2: Fetch user profile using access token.
	profileURL := strings.TrimRight(cfg.CasdoorURL, "/") + "/api/get-account"
	profileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("OIDC: create profile request: %w", err)
	}
	profileReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	profileResp, err := httpClient.Do(profileReq)
	if err != nil {
		return nil, fmt.Errorf("OIDC: profile request: %w", err)
	}
	defer profileResp.Body.Close()

	profileBody, err := io.ReadAll(profileResp.Body)
	if err != nil {
		return nil, fmt.Errorf("OIDC: read profile response: %w", err)
	}

	profile, err := parseCasdoorProfile(profileBody)
	if err != nil {
		return nil, fmt.Errorf("OIDC: parse profile response: %w", err)
	}
	if profile.Sub == "" && profile.Email == "" {
		return nil, fmt.Errorf("OIDC: received empty user profile from Casdoor")
	}
	// Normalize: if sub is missing, fall back to email as identity key.
	if profile.Sub == "" {
		profile.Sub = profile.Email
	}

	// Step 3: Provision or load user.
	// emailVerified is sourced from Casdoor's profile. False forbids linking to
	// existing local accounts (ADR-0033).
	// targetOrgID is "": Casdoor-proxied OIDC carries no org context, so a
	// newly provisioned user joins the instance's organisation.
	userID, orgID, roles, err := s.provisionOIDCUser(ctx, profile.Sub, provider, profile.Email, profile.Name, profile.Avatar, "", profile.EmailVerified, true)
	if err != nil {
		// S22-3: failed OIDC-Provisionierung wird auch persistiert
		s.recordLogin(ctx, "", "", profile.Email, deviceHint, "oidc", "oidc_failed")
		return nil, fmt.Errorf("OIDC: provision user: %w", err)
	}

	authResp, tokenErr := s.issueTokenPair(ctx, userID, orgID, roles, deviceHint, true /* SSO: IdP-authenticated */)
	if tokenErr != nil {
		return authResp, tokenErr
	}
	// S22-3: erfolgreicher OIDC-Login in login_history persistieren.
	s.recordLogin(ctx, orgID, userID, profile.Email, deviceHint, "oidc", "ok")
	return authResp, nil
}

// SAMLLogin processes a SAML assertion consumer response proxied via Casdoor.
// deviceHint is the caller's User-Agent header (truncated to 120 chars).
func (s *Service) SAMLLogin(ctx context.Context, cfg *config.Config, samlResponse, relayState, deviceHint string) (*AuthResponse, error) {
	if cfg.CasdoorURL == "" {
		return nil, ErrCasdoorNotConfigured
	}

	samlURL := strings.TrimRight(cfg.CasdoorURL, "/") + "/api/saml/login"
	samlBody := map[string]string{
		"saml_response": samlResponse,
		"relay_state":   relayState,
	}
	samlBodyJSON, err := json.Marshal(samlBody)
	if err != nil {
		return nil, fmt.Errorf("SAML: marshal request: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, samlURL, bytes.NewReader(samlBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("SAML: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SAML: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("SAML: read response: %w", err)
	}

	var samlResp casdoorSAMLResponse
	if err := json.Unmarshal(respBody, &samlResp); err != nil {
		return nil, fmt.Errorf("SAML: parse response: %w", err)
	}
	if samlResp.Error != "" {
		return nil, fmt.Errorf("SAML: Casdoor error: %s", samlResp.Error)
	}
	// Read the profile through the same envelope-tolerant parse as the OIDC path.
	// This endpoint is a Casdoor controller too, so the flat struct above has the
	// shape R1-07-B07-1 broke on. Unlike the OIDC path this is NOT measured — no
	// SAML flow was driven against a real Casdoor (SA-07 V07r-1) — the parse is
	// simply tolerant of both shapes, and a flat body still lands in samlResp.
	profile, err := parseCasdoorProfile(respBody)
	if err != nil {
		return nil, fmt.Errorf("SAML: parse response: %w", err)
	}
	if profile.Sub != "" {
		samlResp.Sub = profile.Sub
	}
	if profile.Email != "" {
		samlResp.Email = profile.Email
	}
	if profile.Name != "" {
		samlResp.Name = profile.Name
	}
	if samlResp.Sub == "" && samlResp.Email == "" {
		return nil, fmt.Errorf("SAML: received empty user profile from Casdoor")
	}
	if samlResp.Sub == "" {
		samlResp.Sub = samlResp.Email
	}

	// SAML assertions carry an XML-DSig that Casdoor verifies before answering us,
	// so the email is considered IdP-verified.
	userID, orgID, roles, err := s.provisionOIDCUser(ctx, samlResp.Sub, "saml", samlResp.Email, samlResp.Name, "", "", true, true)
	if err != nil {
		// S22-3: failed SAML auch persistieren.
		s.recordLogin(ctx, "", "", samlResp.Email, deviceHint, "saml", "oidc_failed")
		return nil, fmt.Errorf("SAML: provision user: %w", err)
	}

	authResp, tokErr := s.issueTokenPair(ctx, userID, orgID, roles, deviceHint, true /* SSO: IdP-authenticated */)
	if tokErr == nil {
		// S22-3: erfolgreicher SAML-Login.
		s.recordLogin(ctx, orgID, userID, samlResp.Email, deviceHint, "saml", "ok")
	}
	return authResp, tokErr
}

// provisionSAMLUser provisions a user from a direct SAML assertion (S21-1).
// When jitEnabled=false, a missing user causes ErrSAMLUserNotProvisioned.
func (s *Service) provisionSAMLUser(ctx context.Context, orgID, nameID, email, displayName, deviceHint string, jitEnabled bool) (*AuthResponse, error) {
	// Direct SAML assertions are signature-verified by saml_direct.go before
	// this code path is reached, so the email is treated as IdP-verified.
	//
	// orgID comes from the org_saml_configs row the assertion was matched
	// against — the organisation that configured this IdP. It used to be read
	// here and then dropped, and JIT provisioning founded a personal org
	// instead (R1-W2FIX-SSO-01); it is now the org the new user joins.
	userID, resolvedOrgID, roles, err := s.provisionOIDCUser(ctx, nameID, "saml", email, displayName, "", orgID, true, jitEnabled)
	if err != nil {
		s.recordLogin(ctx, orgID, "", email, deviceHint, "saml_direct", "provision_failed")
		return nil, err
	}
	authResp, tokErr := s.issueTokenPair(ctx, userID, resolvedOrgID, roles, deviceHint, true /* SSO: IdP-authenticated */)
	if tokErr == nil {
		s.recordLogin(ctx, resolvedOrgID, userID, email, deviceHint, "saml_direct", "ok")
	}
	return authResp, tokErr
}

// provisionOIDCUser looks up or creates a user based on their OIDC subject.
// It returns the userID, their primary orgID, and the list of role names.
//
// emailVerified must reflect whether the upstream IdP has confirmed ownership
// of the email address. When false, the function refuses to link the OIDC
// subject to a pre-existing local account that happens to share the email —
// this would otherwise allow a trivial account-takeover (ADR-0033).
//
// createIfNotFound controls JIT provisioning: when false, a missing user
// causes ErrSAMLUserNotProvisioned instead of auto-creating an account.
//
// targetOrgID names the organisation a newly provisioned user joins. The direct
// SAML SP path knows it from the org's SAML config; the Casdoor-proxied paths
// pass "" and the instance's organisation is resolved instead. It is never a
// NEW organisation — see resolveInstanceOrg.
func (s *Service) provisionOIDCUser(ctx context.Context, oidcSubject, provider, email, displayName, avatarURL, targetOrgID string, emailVerified, createIfNotFound bool) (string, string, []string, error) {
	// Fail closed on a profile without an email — for every SSO path, since all
	// of them (OIDC, SAML via Casdoor, direct SAML) provision through here.
	// The old behaviour was to create the account with an empty email, which the
	// UNIQUE index then turned into a permanent lock-out for every further
	// email-less login (R1-07-B07-1).
	if email == "" {
		log.Warn().Str("provider", provider).Msg("OIDC: identity provider returned no email address")
		return "", "", nil, ErrOIDCNoEmail
	}

	// Try to find an existing user by OIDC subject.
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE oidc_subject = $1`,
		oidcSubject,
	).Scan(&userID)

	if err != nil {
		// No existing user by subject — try to find by email (may already have a local account).
		emailErr := s.db.QueryRow(ctx,
			`SELECT id::text FROM users WHERE email = $1`,
			email,
		).Scan(&userID)
		if emailErr == nil {
			// Linking would let an unverified-email IdP take over an existing local
			// account.  Refuse unless the IdP has confirmed ownership.
			if !emailVerified {
				log.Warn().Str("provider", provider).Msg("OIDC: refusing to link unverified email to existing account")
				return "", "", nil, ErrEmailNotVerified
			}
			// Link existing user to this OIDC subject.
			if _, updateErr := s.db.Exec(ctx,
				`UPDATE users SET oidc_subject = $1, oidc_provider = $2, avatar_url = COALESCE(NULLIF($3,''), avatar_url), last_login_at = NOW() WHERE id = $4::uuid`,
				oidcSubject, provider, avatarURL, userID,
			); updateErr != nil {
				log.Warn().Err(updateErr).Str("user_id", userID).Msg("failed to link OIDC subject to existing user")
			}
		} else {
			if !createIfNotFound {
				return "", "", nil, ErrSAMLUserNotProvisioned
			}
			// Truly new user — create the account inside the organisation that
			// already exists. Never a fresh one (R1-W2FIX-SSO-01).
			orgID := targetOrgID
			if orgID == "" {
				orgID, err = s.resolveInstanceOrg(ctx)
				if err != nil {
					return "", "", nil, err
				}
			}
			userID, err = s.createOIDCUser(ctx, orgID, oidcSubject, provider, email, displayName, avatarURL)
			if err != nil {
				return "", "", nil, err
			}
		}
	} else {
		// Update last_login_at for existing user.
		if _, updateErr := s.db.Exec(ctx,
			`UPDATE users SET last_login_at = NOW() WHERE id = $1::uuid`, userID,
		); updateErr != nil {
			log.Warn().Err(updateErr).Str("user_id", userID).Msg("failed to update last_login_at")
		}
	}

	// Load org membership.
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
		return "", "", nil, fmt.Errorf("fetch org membership: %w", err)
	}

	return userID, orgID, []string{roleName}, nil
}

// resolveInstanceOrg returns the organisation an SSO user joins when the caller
// does not name one: the oldest, which is the one first-run setup created.
//
// One customer per server, one organisation, one shared ISMS — setup refuses to
// run a second time (setup.IsSetupComplete counts organisations), so the oldest
// row IS the customer's. Additional organisations only exist in demo mode
// (VAKT_DEMO=true), which is not a customer deployment.
//
// A missing organisation is refused, not created: founding the instance is
// first-run setup's job, and letting an IdP login do it is exactly the defect
// this function exists to close.
func (s *Service) resolveInstanceOrg(ctx context.Context) (string, error) {
	var orgID string
	err := s.db.QueryRow(ctx, `
		SELECT id::text FROM organizations
		ORDER BY created_at ASC, id ASC
		LIMIT 1`).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoOrganization
	}
	if err != nil {
		return "", fmt.Errorf("resolve instance organisation: %w", err)
	}
	return orgID, nil
}

// createOIDCUser inserts a new user and joins them to orgID as Viewer.
//
// R1-W2FIX-SSO-01: this used to INSERT a fresh organisation per user, so every SSO
// colleague after the first landed in their own empty management system instead
// of the customer's. Vakt is one customer per server with one shared ISMS; an
// IdP login joins that organisation, it does not found another one.
func (s *Service) createOIDCUser(ctx context.Context, orgID, oidcSubject, provider, email, displayName, avatarURL string) (string, error) {
	if orgID == "" {
		return "", ErrNoOrganization
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var userID string
	// role is set to the least-privilege 'viewer' explicitly (V24-a): SSO users get
	// the Viewer org_members role below, and users.role must match — the default is
	// 'viewer' since migration 249, but relying on the default is the exact D24-1
	// trap, so every insert states the role.
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, display_name, avatar_url, oidc_subject, oidc_provider, is_active, role)
		VALUES ($1, $2, NULLIF($3,''), $4, $5, TRUE, 'viewer')
		RETURNING id::text`,
		email, displayName, avatarURL, oidcSubject, provider,
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("insert OIDC user: %w", err)
	}

	// Assign Viewer role by default for SSO users.
	var roleID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM roles WHERE name = 'Viewer'`).Scan(&roleID)
	if err != nil {
		return "", fmt.Errorf("lookup viewer role: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO org_members (org_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)`,
		orgID, userID, roleID,
	)
	if err != nil {
		return "", fmt.Errorf("insert org member: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	return userID, nil
}
