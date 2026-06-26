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

	"github.com/matharnica/vakt/internal/config"
	"github.com/rs/zerolog/log"
)

// OIDCCallbackInput is sent by the frontend after Casdoor redirects back.
type OIDCCallbackInput struct {
	Code     string `json:"code"     validate:"required"`
	State    string `json:"state"    validate:"required"`
	Provider string `json:"provider" validate:"required,oneof=google github keycloak"`
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

	var profile casdoorUserProfile
	if err := json.Unmarshal(profileBody, &profile); err != nil {
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
	userID, orgID, roles, err := s.provisionOIDCUser(ctx, profile.Sub, provider, profile.Email, profile.Name, profile.Avatar, profile.EmailVerified, true)
	if err != nil {
		// S22-3: failed OIDC-Provisionierung wird auch persistiert
		s.recordLogin(ctx, "", "", profile.Email, deviceHint, "oidc", "oidc_failed")
		return nil, fmt.Errorf("OIDC: provision user: %w", err)
	}

	authResp, tokenErr := s.issueTokenPair(ctx, userID, orgID, roles, deviceHint)
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
	if samlResp.Sub == "" && samlResp.Email == "" {
		return nil, fmt.Errorf("SAML: received empty user profile from Casdoor")
	}
	if samlResp.Sub == "" {
		samlResp.Sub = samlResp.Email
	}

	// SAML assertions carry an XML-DSig that Casdoor verifies before answering us,
	// so the email is considered IdP-verified.
	userID, orgID, roles, err := s.provisionOIDCUser(ctx, samlResp.Sub, "saml", samlResp.Email, samlResp.Name, "", true, true)
	if err != nil {
		// S22-3: failed SAML auch persistieren.
		s.recordLogin(ctx, "", "", samlResp.Email, deviceHint, "saml", "oidc_failed")
		return nil, fmt.Errorf("SAML: provision user: %w", err)
	}

	authResp, tokErr := s.issueTokenPair(ctx, userID, orgID, roles, deviceHint)
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
	userID, resolvedOrgID, roles, err := s.provisionOIDCUser(ctx, nameID, "saml", email, displayName, "", true, jitEnabled)
	if err != nil {
		s.recordLogin(ctx, orgID, "", email, deviceHint, "saml_direct", "provision_failed")
		return nil, err
	}
	authResp, tokErr := s.issueTokenPair(ctx, userID, resolvedOrgID, roles, deviceHint)
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
func (s *Service) provisionOIDCUser(ctx context.Context, oidcSubject, provider, email, displayName, avatarURL string, emailVerified, createIfNotFound bool) (string, string, []string, error) {
	// Try to find an existing user by OIDC subject.
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE oidc_subject = $1`,
		oidcSubject,
	).Scan(&userID)

	if err != nil {
		// No existing user by subject — try to find by email (may already have a local account).
		if email != "" {
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
				// Truly new user — create account.
				userID, err = s.createOIDCUser(ctx, oidcSubject, provider, email, displayName, avatarURL)
				if err != nil {
					return "", "", nil, err
				}
			}
		} else {
			if !createIfNotFound {
				return "", "", nil, ErrSAMLUserNotProvisioned
			}
			// No email available — create with empty email placeholder.
			userID, err = s.createOIDCUser(ctx, oidcSubject, provider, email, displayName, avatarURL)
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

// createOIDCUser inserts a new user and creates their personal organisation.
func (s *Service) createOIDCUser(ctx context.Context, oidcSubject, provider, email, displayName, avatarURL string) (string, error) {
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
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, display_name, avatar_url, oidc_subject, oidc_provider, is_active)
		VALUES ($1, $2, NULLIF($3,''), $4, $5, TRUE)
		RETURNING id::text`,
		email, displayName, avatarURL, oidcSubject, provider,
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("insert OIDC user: %w", err)
	}

	// Derive org name from display name, fall back to email local part, then sub.
	orgName := displayName
	if orgName == "" && email != "" {
		orgName = strings.SplitN(email, "@", 2)[0]
	}
	if orgName == "" {
		orgName = oidcSubject
	}
	orgSlug := slugify(orgName)

	var orgID string
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		RETURNING id::text`,
		orgName, orgSlug,
	).Scan(&orgID)
	if err != nil {
		return "", fmt.Errorf("insert organization: %w", err)
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
