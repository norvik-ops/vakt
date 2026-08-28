// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
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

// AdminResetAuditEntry describes an admin-issued password-reset event for the
// audit trail. It is a local type (not audit.WriteEntry) so the auth package
// stays free of an import on internal/shared/audit — see adminResetAuditFn.
type AdminResetAuditEntry struct {
	OrgID        string
	ActorUserID  string
	TargetUserID string
	TargetEmail  string
	IP           string
	Delivered    bool
}

// adminResetAuditFn records an audit entry for an admin-issued password reset.
// It is a package-level injection seam because the auth package CANNOT import
// internal/shared/audit directly: the audit package imports auth for its HTTP
// route registration, so a direct import would form a cycle (auth ↔ audit).
// The composition root (cmd/api), which imports both, wires this once at
// startup via SetAdminResetAuditWriter, backed by audit.Write. Nil = no audit
// sink (unit tests, or before wiring).
var adminResetAuditFn func(ctx context.Context, e AdminResetAuditEntry)

// SetAdminResetAuditWriter injects the audit sink for admin-issued password
// resets. Call once at startup from the composition root. Safe to leave unset
// (the reset still works; it is simply not recorded to the audit trail).
func SetAdminResetAuditWriter(fn func(ctx context.Context, e AdminResetAuditEntry)) {
	adminResetAuditFn = fn
}

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
//
// It ends BOTH halves of the session, not one:
//
//   - the access token (Paseto, 1 h) goes on the deny-list, so the auth
//     middleware rejects it before it would expire on its own;
//   - every refresh session of that user (30 days) is deleted from
//     refresh_sessions and from Redis, and pw_version is bumped so the stateless
//     access tokens already handed out die on the next request.
//
// R1-W6A-N1 — the second half used to be dead code. This route is mounted on the
// PUBLIC auth group (cmd/api/routes.go: `api.Group("/auth", authRateLimiter)`)
// and always has been, on purpose: an expired token must still be able to end
// its own session. But the old handler took the subject from
// c.Get("user_id") — which only auth middleware ever writes, and there is none
// here. The value was therefore ALWAYS empty and RevokeAllSessions ran NEVER.
// The 30-day refresh session survived every logout. Harmless for the browser
// (the frontend never sees that token) and not harmless at all for anyone
// holding it from the login response.
//
// The subject is now derived from the presented token itself
// (ParseTokenSubjectForRevocation — authentic, expiry not required); the mount
// is untouched. The context is still consulted first so that a future mount
// behind auth middleware keeps working without a second change here.
//
// Rejected alternative: an "optional auth" middleware that fills in user_id when
// the token validates and lets everything else through. It fails the case this
// fix is for — the ordinary logout from an idle tab presents an EXPIRED access
// token over a live refresh session, so the middleware would find nothing to
// set and leave the long-lived session standing, exactly as before. Mounting the
// route behind the real auth middleware was rejected for the reason the mount is
// public in the first place: it would answer 401 to everyone whose token already
// expired, i.e. deny logout to the people most in need of it.
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

	// The local teardown happens on every exit path below, success or not: a
	// browser that asked to leave should stop presenting the token even when the
	// server could not revoke it. It is the least-harm half of the job, and the
	// status code carries the rest of the truth.
	//
	// Set here rather than in a defer: SetCookie only adds a header, and headers
	// are flushed by the first c.JSON — a deferred call would run after
	// WriteHeader and silently drop both Set-Cookie lines.
	h.clearSessionCookies(c)

	// R1-SA13-02: Die Echtheitspruefung steht jetzt VOR dem Widerruf.
	//
	// Vorher lief RevokeToken als Erstes — und RevokeToken prueft nichts, es
	// schreibt den Schluessel in Redis UND in die PG-Ausweichtabelle. Ein
	// beliebiger Bearer-String landete damit im Widerrufsspeicher, obwohl der
	// Handler danach 401 antwortete: ein unauthentifiziert befuellbarer Speicher.
	//
	// Die Umstellung ist gefahrlos, weil ParseTokenSubjectForRevocation mit
	// NewParserWithoutExpiryCheck arbeitet: es prueft Signatur und Schluessel,
	// ueberspringt aber die Ablaufpruefung. Ein Token, das dieser Server
	// ausgestellt hat, kommt also auch abgelaufen durch — genau das, was ein
	// Widerruf braucht. Ein gefaelschtes kommt nicht durch, und fuer das gibt es
	// auch nichts zu widerrufen.
	userID, _ := c.Get("user_id").(string)
	if userID == "" {
		subject, err := ParseTokenSubjectForRevocation(h.service.key, tokenStr)
		if err != nil {
			// Not a token this server minted (forged, truncated, or issued under a
			// key that has since been rotated). There is no session to identify,
			// so there is nothing further to revoke — and saying "logged out"
			// would claim work that never happened. 4xx is also the honest signal
			// for the client: the server looked at the token and did not accept
			// it, which the frontend treats as a harmless already-dead session
			// rather than an unconfirmed revocation.
			log.Debug().Err(err).Msg("logout: token carries no usable subject")
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "token not recognised — nothing to revoke",
				"code":  "AUTH_INVALID_TOKEN",
			})
		}
		userID = subject
	}

	// R1-W7A-N3: RevokeToken used to return a hard nil and this used to answer
	// 200 regardless. A revocation that reached neither Redis nor the PG fallback
	// left the token good for the rest of its hour while the user was told they
	// were signed out.
	revokeErr := h.service.RevokeToken(c.Request().Context(), tokenStr)
	if revokeErr != nil {
		log.Error().Err(revokeErr).Msg("logout: access-token revocation failed")
	}

	// Revoke all refresh sessions so a stolen refresh token cannot be used
	// after logout (AUTH-001: refresh sessions were not cleaned up on logout).
	sessionErr := h.service.RevokeAllSessions(c.Request().Context(), userID)
	if sessionErr != nil {
		log.Error().Err(sessionErr).Str("user_id", userID).Msg("logout: session revocation failed")
	}

	if revokeErr != nil || sessionErr != nil {
		// 5xx on purpose, and it is the contract the frontend already reads:
		// TopBar.tsx treats >=500 (and a dropped connection) as "revocation
		// unconfirmed" and warns the user that the session may still be open,
		// while 4xx stays silent. A failed revocation belongs in the first
		// bucket. The cookies are cleared by the deferred teardown all the same.
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "the session could not be revoked on the server — it may still be valid",
			"code":  "LOGOUT_REVOCATION_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "logged out"})
}

// clearSessionCookies expires the httpOnly access token cookie and the CSRF
// cookie. Runs on every Logout exit path, including the failing ones.
func (h *Handler) clearSessionCookies(c echo.Context) {
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

	// S121-F4 (F1-Auth): the pure per-email lockout that used to sit here has been
	// removed. It locked an account after 5 failures from ANY IP, so anyone could
	// deny service to any user just by knowing their address — the exact opposite
	// of the NAT-safe guarantee the (IP, email) scheme (S107/ADR-0044) exists to
	// provide, and it was never part of the documented design. Targeted brute-force
	// is covered by the (IP, email) lockout above (10) and credential spraying by
	// the per-IP lockout (50), both enforced before we get here.

	loginDeviceHint := c.Request().Header.Get("User-Agent")
	if len(loginDeviceHint) > 120 {
		loginDeviceHint = loginDeviceHint[:120]
	}
	resp, err := h.service.Login(c.Request().Context(), body.Email, body.Password, loginDeviceHint)
	if err != nil {
		log.Debug().Err(err).Str("email_redacted", logsafe.RedactEmail(body.Email)).Msg("login failed")
		// Record the failure for the primary (IP, email) lockout and the secondary
		// per-IP lockout. No pure per-email counter — see S121-F4 note above.
		h.service.recordIPEmailLoginFailure(c.Request().Context(), clientIP, body.Email)
		h.service.recordIPLoginFailure(c.Request().Context(), clientIP)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid credentials",
			"code":  "AUTH_INVALID_CREDENTIALS",
		})
	}

	// Successful login — clear this user's (IP, email) failure counter so their own
	// typos don't count against them on the next attempt. S121-F4: this used to
	// clear only the (now removed) pure per-email counter, leaving the (IP, email)
	// counter standing after a successful login — a user who mistyped a few times
	// and then got in could still be locked out by their next single typo.
	//
	// The per-IP counter is intentionally NOT cleared here: clearing it on any
	// successful login from that IP would let an attacker reset a near-threshold
	// IP counter by piggybacking on a legitimate login from the same network.
	// The ExpireNX fix ensures the 15-min TTL runs from the first failure and
	// expires naturally without any explicit clear needed.
	h.service.clearLoginFailures(c.Request().Context(), clientIP, body.Email)

	// S124-1 (SA14-01): two-stage login. When the account has TOTP enabled, Login
	// returns no session — only a short-lived mfa_pending token. The client must
	// POST it plus a TOTP/backup code to /auth/2fa/login-verify. Do NOT set the
	// access cookie here; a stolen password must not yield a session.
	if resp.MFARequired {
		// resp already carries {mfa_required, mfa_token, user}; the omitempty tags
		// keep access_token/refresh_token/expires_in out of the body.
		return c.JSON(http.StatusOK, resp)
	}

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
		// orgid-lint: global — caller's own role list across their own org memberships (user_id from auth token)
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
	// R1-07-B07-3: refuse a provider the callback cannot complete, here, before
	// the user is sent to a foreign identity provider. Otherwise they sign in
	// there and the request fails afterwards with 422 — at the one point in the
	// flow where the user can do nothing about it.
	if !isSupportedOIDCProvider(provider) {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "unsupported OIDC provider: " + provider + " (supported: " + strings.Join(OIDCProviders, ", ") + ")",
			"code":  "AUTH_OIDC_PROVIDER_UNSUPPORTED",
		})
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

	// R1-07-B04: Casdoor answers its own controller envelope — HTTP 200 with a
	// JSON body {"status":"error",...} — when the app id is unknown. The status
	// check above passes, and blobbing that body out under application/xml
	// handed a consuming IdP broken XML with a success status. Measured live: a
	// JSON error under Content-Type: application/xml, HTTP 200.
	//
	// Serve the body only if it is what the content type claims.
	if !isSAMLMetadataDocument(xmlBody) {
		log.Error().
			Str("url", metadataURL).
			Int("bytes", len(xmlBody)).
			Msg("saml_metadata: Casdoor answered 200 with something that is not SAML metadata")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "Casdoor did not return SAML metadata — check the SAML application id in Casdoor",
			"code":  "AUTH_SAML_UPSTREAM_ERROR",
		})
	}

	return c.Blob(http.StatusOK, "application/xml", xmlBody)
}

// isSAMLMetadataDocument reports whether body's first XML element is a SAML
// metadata root (EntityDescriptor or EntitiesDescriptor).
//
// It only looks at the root element: validating the whole document is the
// consuming IdP's job, and a stricter check here would reject metadata this
// code has no business having an opinion about. Entity expansion is disabled
// and the input is size-capped, so a hostile upstream cannot turn this into a
// Billion-Laughs (same treatment as buildSAMLSP, SEC-M03).
func isSAMLMetadataDocument(body []byte) bool {
	if len(body) == 0 || len(body) > samlMetadataMaxBytes {
		return false
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Entity = map[string]string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local == "EntityDescriptor" ||
				start.Name.Local == "EntitiesDescriptor"
		}
	}
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

// adminResetResponse is the response body for an admin-issued password reset.
// The raw reset token is deliberately ABSENT (R1-24-RT01): only the delivery
// status is exposed. Reason is populated only when sent=false.
type adminResetResponse struct {
	Sent   bool   `json:"sent"`
	Reason string `json:"reason,omitempty"`
}

// AdminGeneratePasswordResetToken handles POST /api/v1/admin/users/:email/password-reset-token.
// Admin-only endpoint that issues a password reset for a member of the CALLER'S
// OWN organisation. The reset link is delivered by email — the raw token is NEVER
// returned in the response body.
//
// SECURITY (R1-24-RT01 / R1-10-V02): before this fix the handler looked up the
// target by email alone (globally unique) and returned the reset link — with
// only a same-org Admin role check — which let an admin of one org mint a reset
// token for ANY account platform-wide and take it over (full cross-org account
// takeover, live-confirmed against the DB). The org scope now runs inside
// AdminIssuePasswordReset (JOIN on org_members keyed on the caller's org), and
// the token no longer leaves the server.
func (h *Handler) AdminGeneratePasswordResetToken(c echo.Context) error {
	email := c.Param("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "email is required",
			"code":  "AUTH_BAD_REQUEST",
		})
	}

	// The caller's org scopes the lookup. This is populated by AuthMiddleware on
	// the protected chain; an empty value would let the query resolve nothing —
	// but treat it as a hard failure rather than silently issuing across orgs.
	callerOrgID, _ := c.Get("org_id").(string)
	callerUserID, _ := c.Get("user_id").(string)
	if callerOrgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_TOKEN",
		})
	}

	frontendURL, smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom := "", "", "", "", "", ""
	if h.cfg != nil {
		frontendURL = h.cfg.FrontendURL
		smtpHost = h.cfg.SMTPHost
		smtpPort = h.cfg.SMTPPort
		smtpUser = h.cfg.SMTPUser
		smtpPass = h.cfg.SMTPPass
		smtpFrom = h.cfg.SMTPFrom
	}

	targetUserID, sent, err := h.service.AdminIssuePasswordReset(
		c.Request().Context(), email, callerOrgID,
		frontendURL, smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom,
	)
	if err != nil {
		log.Error().Err(err).Str("email_redacted", logsafe.RedactEmail(email)).Msg("admin: issue password reset failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to issue password reset",
			"code":  "AUTH_RESET_GENERATE_FAILED",
		})
	}
	if targetUserID == "" {
		// No such active user in the caller's org. Same shape as a genuine
		// not-found: an org2 admin probing an org1 address gets a 404, never a token.
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "user not found",
			"code":  "AUTH_USER_NOT_FOUND",
		})
	}

	// Audit the admin-issued reset — same class as the usermgmt break-glass
	// routes (MFA reset, role change). Fire-and-forget via the injected sink;
	// never blocks the reset. See adminResetAuditFn for why this is a hook and
	// not a direct audit.Write call.
	if adminResetAuditFn != nil {
		adminResetAuditFn(c.Request().Context(), AdminResetAuditEntry{
			OrgID:        callerOrgID,
			ActorUserID:  callerUserID,
			TargetUserID: targetUserID,
			TargetEmail:  email,
			IP:           c.RealIP(),
			Delivered:    sent,
		})
	}

	if !sent {
		// The user exists (200, not 404), but no reset email went out — either
		// SMTP is not configured, or delivery failed. Either way the token is
		// deliberately NOT returned in the body (that was the takeover primitive).
		reason := "delivery_failed"
		if smtpHost == "" {
			reason = "smtp_not_configured"
		}
		return c.JSON(http.StatusOK, adminResetResponse{Sent: false, Reason: reason})
	}
	return c.JSON(http.StatusOK, adminResetResponse{Sent: true})
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
