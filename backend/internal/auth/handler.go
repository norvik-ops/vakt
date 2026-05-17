// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/sechealth-app/sechealth/internal/config"
)

// samlHTTPClient is used for fetching SAML metadata from Casdoor.
// A 15-second timeout prevents hanging requests to unresponsive IdP endpoints.
var samlHTTPClient = &http.Client{Timeout: 15 * time.Second}

// Handler holds HTTP handler methods for the auth endpoints.
type Handler struct {
	service  *Service
	validate *validator.Validate
	cfg      *config.Config
}

// NewHandler constructs an auth Handler.
func NewHandler(service *Service, cfg *config.Config) *Handler {
	return &Handler{
		service:  service,
		validate: validator.New(),
		cfg:      cfg,
	}
}

// Logout handles POST /api/v1/auth/logout.
// It reads the Paseto token from the Authorization header, hashes it with
// SHA-256, and stores the hash in Redis with a TTL equal to the remaining
// token lifetime so that AuthMiddleware can reject the token even before it
// naturally expires.
func (h *Handler) Logout(c echo.Context) error {
	header := c.Request().Header.Get("Authorization")
	const prefix = "Bearer "
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
			"error": err.Error(),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	resp, err := h.service.Register(c.Request().Context(), input)
	if err != nil {
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
		Password string `json:"password" validate:"required,min=8,max=72"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	// Account lockout: reject immediately if too many recent failures.
	locked, lockErr := h.service.checkAccountLocked(c.Request().Context(), body.Email)
	if lockErr != nil {
		log.Warn().Err(lockErr).Str("email", body.Email).Msg("login: lockout check error")
	}
	if locked {
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "Account temporarily locked. Try again in 15 minutes.",
			"code":  "ACCOUNT_LOCKED",
		})
	}

	resp, err := h.service.Login(c.Request().Context(), body.Email, body.Password)
	if err != nil {
		log.Debug().Err(err).Str("email", body.Email).Msg("login failed")
		// Record failure to enable lockout after repeated bad credentials.
		h.service.recordLoginFailure(c.Request().Context(), body.Email)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid credentials",
			"code":  "AUTH_INVALID_CREDENTIALS",
		})
	}

	// Successful login — clear any accumulated failure counter.
	h.service.clearLoginFailures(c.Request().Context(), body.Email)
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
			"error": err.Error(),
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
	return c.JSON(http.StatusOK, resp)
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
			"error": err.Error(),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	resp, err := h.service.OIDCLogin(c.Request().Context(), h.cfg, input.Provider, input.Code, input.State)
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
			"error": err.Error(),
			"code":  "AUTH_VALIDATION_ERROR",
		})
	}

	resp, err := h.service.SAMLLogin(c.Request().Context(), h.cfg, input.SAMLResponse, input.RelayState)
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
		log.Error().Err(err).Str("email", body.Email).Msg("password reset request failed")
	}
	// Always 200.
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ResetPassword handles POST /api/v1/auth/password-reset/confirm.
func (h *Handler) ResetPassword(c echo.Context) error {
	var body struct {
		Token    string `json:"token"    validate:"required"`
		Password string `json:"password" validate:"required,min=8,max=72"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "AUTH_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(body); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
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
		log.Error().Err(err).Msg("password reset confirm failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Passwort konnte nicht zurückgesetzt werden",
			"code":  "AUTH_RESET_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
