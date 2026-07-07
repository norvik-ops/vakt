// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// weakPasswordCode is the error code returned to clients when a password does
// not satisfy the platform complexity requirements.
const weakPasswordCode = "AUTH_WEAK_PASSWORD"

// humanValidationError converts a go-playground/validator error into a
// user-facing German message. Raw validator strings (e.g. "Key: 'Password'
// Error:Field validation for 'Password' failed on the 'min' tag") must never
// reach the UI — this function maps the most common tags to natural language.
func humanValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "Eingabe ungültig. Bitte alle Felder korrekt ausfüllen."
	}
	for _, fe := range ve {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			switch field {
			case "email":
				return "E-Mail-Adresse ist erforderlich."
			case "password":
				return "Passwort ist erforderlich."
			default:
				return "Pflichtfeld fehlt: " + fe.Field()
			}
		case "email":
			return "Keine gültige E-Mail-Adresse."
		case "min":
			switch field {
			case "password":
				return fmt.Sprintf("Passwort muss mindestens %s Zeichen lang sein.", fe.Param())
			case "name", "display_name":
				return fmt.Sprintf("%s muss mindestens %s Zeichen lang sein.", fe.Field(), fe.Param())
			}
		case "max":
			switch field {
			case "password":
				return fmt.Sprintf("Passwort darf maximal %s Zeichen lang sein.", fe.Param())
			}
		}
	}
	return "Eingabe ungültig. Bitte alle Felder korrekt ausfüllen."
}

// samlHTTPClient is used for fetching SAML metadata from Casdoor.
// A 15-second timeout prevents hanging requests to unresponsive IdP endpoints.
var samlHTTPClient = &http.Client{Timeout: 15 * time.Second}

// Handler holds HTTP handler methods for the auth endpoints.
type Handler struct {
	service  *Service
	validate *validator.Validate
	cfg      *config.Config
	db       *pgxpool.Pool // for SAML direct SP config lookups
}

// NewHandler constructs an auth Handler.
func NewHandler(service *Service, cfg *config.Config) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
		cfg:      cfg,
	}
}

// WithDB attaches a DB pool to the handler (required for direct SAML SP).
func (h *Handler) WithDB(db *pgxpool.Pool) *Handler {
	h.db = db
	return h
}

// Logout handles POST /api/v1/auth/logout.
// It reads the Paseto token from the Authorization header or the httpOnly
// cookie, hashes it with SHA-256, and stores the hash in Redis with a TTL
// equal to the remaining token lifetime so that AuthMiddleware can reject
// the token even before it naturally expires.
func (h *Handler) Logout(c echo.Context) error {
	header := c.Request().Header.Get("Authorization")
	const prefix = "Bearer "

	// Accept token from cookie when no Authorization header is present.
	if header == "" {
		if cookie, err := c.Cookie("access_token"); err == nil && cookie.Value != "" {
			header = prefix + cookie.Value
		}
	}

	if len(header) <= len(prefix) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "missing authorization header",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	tokenStr := header[len(prefix):]

	if err := h.service.RevokeToken(c.Request().Context(), tokenStr); err != nil {
		log.Error().Err(err).Msg("logout: revoke token failed")
		// Still return 200 — the token will expire naturally.
	}

	// Revoke all refresh sessions so a stolen refresh token cannot be used
	// after logout (AUTH-001: refresh sessions were not cleaned up on logout).
	userID, _ := c.Get("user_id").(string)
	if userID != "" {
		if err := h.service.RevokeAllSessions(c.Request().Context(), userID); err != nil {
			log.Warn().Err(err).Msg("logout: revoke sessions failed")
			// non-fatal: access token is already revoked
		}
	}

	// Clear the httpOnly access token cookie.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    "",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   -1,
	})
	ClearCSRFCookie(c)

	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}

// Register handles POST /api/v1/auth/register.
func (h *Handler) Register(c echo.Context) error {
	var input RegisterInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	deviceHint := c.Request().Header.Get("User-Agent")
	if len(deviceHint) > 120 {
		deviceHint = deviceHint[:120]
	}
	resp, err := h.service.Register(c.Request().Context(), input, deviceHint)
	if err != nil {
		if errors.Is(err, ErrRegistrationDisabled) {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "registration is disabled",
				"code":  "AUTH_REGISTRATION_DISABLED",
			})
		}
		if errors.Is(err, ErrWeakPassword) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": err.Error(),
				"code":  weakPasswordCode,
			})
		}
		log.Error().Err(err).Msg("register failed")
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "registration failed",
			"code":  "AUTH_REGISTER_FAILED",
		})
	}
	return c.JSON(http.StatusCreated, resp)
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c echo.Context) error {
	var body struct {
		Email    string `json:"email"    validate:"required,email"`
		Password string `json:"password" validate:"required,min=10,max=72"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	clientIP := c.RealIP()

	// Secondary IP-level lockout: reject if this IP has exceeded the threshold
	// across ANY email address (credential-spraying defense). Threshold is
	// configurable via VAKT_RATELIMIT_IP_MAX (default 50) — high enough that
	// shared NAT isn't a problem under normal circumstances.
	ipLocked, ipLockErr := h.service.checkIPLocked(c.Request().Context(), clientIP)
	if errors.Is(ipLockErr, ErrLockoutCheckUnavailable) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Authentication temporarily unavailable. Please retry shortly.",
			"code":  "AUTH_LOCKOUT_UNAVAILABLE",
		})
	}
	if ipLockErr != nil {
		log.Warn().Err(ipLockErr).Str("ip", clientIP).Msg("login: IP lockout check error")
	}
	if ipLocked {
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Too many failed attempts from this IP. Try again in 15 minutes.",
			"code":  "IP_LOCKED",
		})
	}

	// Primary (IP, email) lockout: blocks targeted brute-force of one account
	// without locking out other users behind the same NAT/VPN.
	ipEmailLocked, ipEmailLockErr := h.service.checkIPEmailLocked(c.Request().Context(), clientIP, body.Email)
	if errors.Is(ipEmailLockErr, ErrLockoutCheckUnavailable) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Authentication temporarily unavailable. Please retry shortly.",
			"code":  "AUTH_LOCKOUT_UNAVAILABLE",
		})
	}
	if ipEmailLockErr != nil {
		log.Warn().Err(ipEmailLockErr).Str("ip", clientIP).Msg("login: IP+email lockout check error")
	}
	if ipEmailLocked {
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Account temporarily locked. Try again in 15 minutes.",
			"code":  "ACCOUNT_LOCKED",
		})
	}

	// Account lockout: reject immediately if too many recent failures for this email.
	locked, lockErr := h.service.checkAccountLocked(c.Request().Context(), body.Email)
	if errors.Is(lockErr, ErrLockoutCheckUnavailable) {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Authentication temporarily unavailable. Please retry shortly.",
			"code":  "AUTH_LOCKOUT_UNAVAILABLE",
		})
	}
	if lockErr != nil {
		log.Warn().Err(lockErr).Str("email_redacted", logsafe.RedactEmail(body.Email)).Msg("login: lockout check error")
	}
	if locked {
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Account temporarily locked. Try again in 15 minutes.",
			"code":  "ACCOUNT_LOCKED",
		})
	}

	loginDeviceHint := c.Request().Header.Get("User-Agent")
	if len(loginDeviceHint) > 120 {
		loginDeviceHint = loginDeviceHint[:120]
	}
	resp, err := h.service.Login(c.Request().Context(), body.Email, body.Password, loginDeviceHint)
	if err != nil {
		log.Debug().Err(err).Str("email_redacted", logsafe.RedactEmail(body.Email)).Msg("login failed")
		// Record failure for per-email, per-(IP,email), and secondary per-IP lockouts.
		h.service.recordLoginFailure(c.Request().Context(), body.Email)
		h.service.recordIPEmailLoginFailure(c.Request().Context(), clientIP, body.Email)
		h.service.recordIPLoginFailure(c.Request().Context(), clientIP)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid credentials",
			"code":  "AUTH_INVALID_CREDENTIALS",
		})
	}

	// Successful login — clear per-email failure counter so the user's own
	// typos don't count against them on the next attempt.
	// The per-IP counter is intentionally NOT cleared here: clearing it on any
	// successful login from that IP would let an attacker reset a near-threshold
	// IP counter by piggybacking on a legitimate login from the same network.
	// The ExpireNX fix ensures the 15-min TTL runs from the first failure and
	// expires naturally without any explicit clear needed.
	h.service.clearLoginFailures(c.Request().Context(), body.Email)

	// Set access token as httpOnly cookie (XSS protection).
	// SameSite=Strict + double-submit CSRF token cookie prevent CSRF.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    resp.AccessToken,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600, // 1 hour, matches access token TTL
	})
	csrfToken := GenerateCSRFToken()
	SetCSRFCookie(c, csrfToken)
	resp.CSRFToken = csrfToken

	return c.JSON(http.StatusOK, resp)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handler) Refresh(c echo.Context) error {
	var body struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	resp, err := h.service.Refresh(c.Request().Context(), body.RefreshToken)
	if err != nil {
		log.Debug().Err(err).Msg("token refresh failed")
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid or expired refresh token",
			"code":  "AUTH_INVALID_REFRESH_TOKEN",
		})
	}

	// Rotate the httpOnly access token cookie and CSRF token on every refresh.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    resp.AccessToken,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600, // 1 hour, matches access token TTL
	})
	csrfToken := GenerateCSRFToken()
	SetCSRFCookie(c, csrfToken)
	resp.CSRFToken = csrfToken

	return c.JSON(http.StatusOK, resp)
}

// Me handles GET /api/v1/auth/me. Returns the current user's identity for
// the front-end to hydrate its auth store after a page reload, replacing the
// previous localStorage-based snapshot (audit F032: no PII in localStorage).
// Requires authentication — mounted on the `protected` group in cmd/api.
func (h *Handler) Me(c echo.Context) error {
	userID, _ := c.Get("user_id").(string)
	if userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthenticated",
			"code":  "AUTH_UNAUTHENTICATED",
		})
	}
	ctx := c.Request().Context()
	var user AuthUser
	err := h.service.db.QueryRow(ctx, `
		SELECT id::text, email, COALESCE(display_name, email)
		FROM users WHERE id = $1::uuid`, userID).
		Scan(&user.ID, &user.Email, &user.DisplayName)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthenticated",
			"code":  "AUTH_UNAUTHENTICATED",
		})
	}
	if rolesAny, ok := c.Get("roles").([]string); ok && len(rolesAny) > 0 {
		user.Roles = rolesAny
	} else {
		rows, qErr := h.service.db.Query(ctx, `
			SELECT r.name FROM org_members om
			JOIN roles r ON r.id = om.role_id
			WHERE om.user_id = $1::uuid
			ORDER BY om.joined_at ASC`, userID)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if scanErr := rows.Scan(&name); scanErr == nil {
					user.Roles = append(user.Roles, name)
				}
			}
		}
	}
	// Echo the current csrf_token cookie value back in the body (see
	// AuthResponse.CSRFToken) so the frontend can rehydrate its in-memory
	// fallback after a page reload, not just right after login/refresh.
	resp := MeResponse{AuthUser: user}
	if cookie, err := c.Cookie(CSRFCookieName); err == nil {
		resp.CSRFToken = cookie.Value
	}
	return c.JSON(http.StatusOK, resp)
}

// MeResponse extends AuthUser with the current CSRF token for the frontend's
// in-memory cache (see AuthResponse.CSRFToken for the rationale).
type MeResponse struct {
	AuthUser
	CSRFToken string `json:"csrf_token,omitempty"`
}

// OIDCInitiate handles GET /api/v1/auth/oidc/initiate.
// Generates a cryptographically random state, stores it in Redis with a 10-minute TTL,
// and returns the Casdoor authorization URL with the state embedded (OAuth2 CSRF protection).
func (h *Handler) OIDCInitiate(c echo.Context) error {
	provider := c.QueryParam("provider")
	if provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider required"})
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "state generation failed"})
	}
	state := hex.EncodeToString(raw)

	ctx := c.Request().Context()
	if err := h.service.StoreOIDCState(ctx, state); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "state storage failed"})
	}

	casdoorURL := ""
	clientID := ""
	frontendURL := ""
	if h.cfg != nil {
		casdoorURL = h.cfg.CasdoorURL
		clientID = h.cfg.CasdoorClientID
		frontendURL = h.cfg.FrontendURL
	}
	if casdoorURL == "" {
		return c.JSON(http.StatusNotImplemented, map[string]string{
			"error": "OIDC not configured",
			"code":  "AUTH_OIDC_NOT_CONFIGURED",
		})
	}

	redirectURI := strings.TrimRight(frontendURL, "/") + "/auth/callback"
	redirectURL := strings.TrimRight(casdoorURL, "/") + "/login/oauth/authorize?" +
		"client_id=" + clientID +
		"&response_type=code" +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&scope=openid+profile+email" +
		"&state=" + state

	return c.JSON(http.StatusOK, map[string]string{
		"state":        state,
		"redirect_url": redirectURL,
	})
}

// OIDCCallback handles POST /api/v1/auth/oidc/callback.
// It receives an OAuth2 authorization code from the frontend after Casdoor redirects
// back, exchanges it for a Paseto token pair, and provisions the user on first login.
func (h *Handler) OIDCCallback(c echo.Context) error {
	var input OIDCCallbackInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	if err := h.service.ValidateAndConsumeOIDCState(c.Request().Context(), input.State); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid state parameter",
			"code":  "AUTH_INVALID_STATE",
		})
	}

	oidcDeviceHint := c.Request().Header.Get("User-Agent")
	if len(oidcDeviceHint) > 120 {
		oidcDeviceHint = oidcDeviceHint[:120]
	}
	resp, err := h.service.OIDCLogin(c.Request().Context(), h.cfg, input.Provider, input.Code, input.State, oidcDeviceHint)
	if err != nil {
		if errors.Is(err, ErrCasdoorNotConfigured) {
			return c.JSON(http.StatusNotImplemented, map[string]string{
				"error": err.Error(),
				"code":  "AUTH_OIDC_NOT_CONFIGURED",
			})
		}
		log.Error().Err(err).Str("provider", input.Provider).Msg("OIDC login failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "OIDC login failed",
			"code":  "AUTH_OIDC_FAILED",
		})
	}

	// Set access token as httpOnly cookie — same policy as password login.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    resp.AccessToken,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600,
	})
	csrfToken := GenerateCSRFToken()
	SetCSRFCookie(c, csrfToken)
	resp.CSRFToken = csrfToken

	return c.JSON(http.StatusOK, resp)
}

// SAMLCallback handles POST /api/v1/auth/saml/callback (assertion consumer endpoint).
func (h *Handler) SAMLCallback(c echo.Context) error {
	var input SAMLCallbackInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	samlDeviceHint := c.Request().Header.Get("User-Agent")
	if len(samlDeviceHint) > 120 {
		samlDeviceHint = samlDeviceHint[:120]
	}
	resp, err := h.service.SAMLLogin(c.Request().Context(), h.cfg, input.SAMLResponse, input.RelayState, samlDeviceHint)
	if err != nil {
		if errors.Is(err, ErrCasdoorNotConfigured) {
			return c.JSON(http.StatusNotImplemented, map[string]string{
				"error": err.Error(),
				"code":  "AUTH_SAML_NOT_CONFIGURED",
			})
		}
		log.Error().Err(err).Msg("SAML login failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "SAML login failed",
			"code":  "AUTH_SAML_FAILED",
		})
	}

	// Set access token as httpOnly cookie — same policy as password login.
	secure := CookieSecure(c)
	c.SetCookie(&http.Cookie{ // nosemgrep: cookie-missing-secure -- Secure is set via variable; static analysis can't resolve it
		Name:     "access_token",
		Value:    resp.AccessToken,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1",
		MaxAge:   3600,
	})
	csrfToken := GenerateCSRFToken()
	SetCSRFCookie(c, csrfToken)
	resp.CSRFToken = csrfToken

	return c.JSON(http.StatusOK, resp)
}

// SAMLMetadata handles GET /api/v1/auth/saml/metadata.
// Fetches the SP metadata XML from the configured Casdoor instance and proxies
// it back to the client so that IdPs can consume it directly.
func (h *Handler) SAMLMetadata(c echo.Context) error {
	if h.cfg == nil || h.cfg.CasdoorURL == "" {
		return c.JSON(http.StatusNotImplemented, map[string]string{
			"error": "SAML: configure CASDOOR_URL env var",
			"code":  "AUTH_SAML_NOT_CONFIGURED",
		})
	}

	// Casdoor exposes SP metadata at GET /api/saml/metadata?id=<app-id>.
	// The app-id defaults to the configured ClientID when no explicit override exists.
	appID := h.cfg.CasdoorClientID
	metadataURL := fmt.Sprintf("%s/api/saml/metadata?id=%s",
		h.cfg.CasdoorURL, appID)

	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, metadataURL, nil)
	if err != nil {
		log.Error().Err(err).Str("url", metadataURL).Msg("saml_metadata: build request failed")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "failed to build Casdoor metadata request",
			"code":  "AUTH_SAML_UPSTREAM_ERROR",
		})
	}

	resp, err := samlHTTPClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", metadataURL).Msg("saml_metadata: Casdoor not reachable")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "Casdoor not reachable — check CASDOOR_URL",
			"code":  "AUTH_SAML_UPSTREAM_UNREACHABLE",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("url", metadataURL).
			Int("status", resp.StatusCode).
			Msg("saml_metadata: Casdoor returned non-200")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("Casdoor returned HTTP %d for metadata", resp.StatusCode),
			"code":  "AUTH_SAML_UPSTREAM_ERROR",
		})
	}

	xmlBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("saml_metadata: read Casdoor response failed")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "failed to read Casdoor metadata response",
			"code":  "AUTH_SAML_UPSTREAM_ERROR",
		})
	}

	return c.Blob(http.StatusOK, "application/xml", xmlBody)
}

// SAMLACS handles POST /api/v1/auth/saml/acs (assertion consumer service, alias).
func (h *Handler) SAMLACS(c echo.Context) error {
	return h.SAMLCallback(c)
}

// RequestPasswordReset handles POST /api/v1/auth/password-reset/request.
// Always returns 200 to avoid leaking whether an email address exists.
func (h *Handler) RequestPasswordReset(c echo.Context) error {
	var body struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		// Still return 200 — no detail exposed.
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	frontendURL := ""
	smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom := "", "", "", "", ""
	if h.cfg != nil {
		frontendURL = h.cfg.FrontendURL
		smtpHost = h.cfg.SMTPHost
		smtpPort = h.cfg.SMTPPort
		smtpUser = h.cfg.SMTPUser
		smtpPass = h.cfg.SMTPPass
		smtpFrom = h.cfg.SMTPFrom
	}

	if err := h.service.RequestPasswordReset(
		c.Request().Context(),
		body.Email,
		frontendURL,
		smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom,
	); err != nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(body.Email)).Msg("password reset request failed")
	}
	// Always 200.
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// AdminGeneratePasswordResetToken handles POST /api/v1/admin/users/:email/password-reset-token.
// Admin-only endpoint that generates a password reset link without requiring SMTP.
func (h *Handler) AdminGeneratePasswordResetToken(c echo.Context) error {
	email := c.Param("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "email is required",
			"code":  "AUTH_BAD_REQUEST",
		})
	}

	frontendURL := ""
	if h.cfg != nil {
		frontendURL = h.cfg.FrontendURL
	}

	resetLink, err := h.service.GeneratePasswordResetLink(c.Request().Context(), email, frontendURL)
	if err != nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: generate password reset link failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate reset link",
			"code":  "AUTH_RESET_GENERATE_FAILED",
		})
	}
	if resetLink == "" {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "user not found",
			"code":  "AUTH_USER_NOT_FOUND",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"reset_link": resetLink,
		"expires_in": "1h",
	})
}

// ResetPassword handles POST /api/v1/auth/password-reset/confirm.
func (h *Handler) ResetPassword(c echo.Context) error {
	var body struct {
		Token    string `json:"token"    validate:"required"`
		Password string `json:"password" validate:"required,min=10,max=72"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": humanValidationError(err),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	if err := h.service.ResetPassword(c.Request().Context(), body.Token, body.Password); err != nil {
		if errors.Is(err, ErrTokenInvalid) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Link ungültig oder abgelaufen",
				"code":  "AUTH_RESET_TOKEN_INVALID",
			})
		}
		if errors.Is(err, ErrWeakPassword) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": err.Error(),
				"code":  weakPasswordCode,
			})
		}
		log.Error().Err(err).Msg("password reset confirm failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Passwort konnte nicht zurückgesetzt werden",
			"code":  "AUTH_RESET_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
