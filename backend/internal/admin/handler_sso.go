package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ─── S21-1: SAML Direct SP Config ────────────────────────────────────────────

// GetOrgSAMLConfigResponse is the public view of OrgSAMLConfig (key PEM omitted).
type GetOrgSAMLConfigResponse struct {
	OrgID           string `json:"org_id"`
	EntityID        string `json:"entity_id"`
	ACSURL          string `json:"acs_url"`
	IDPMetadata     string `json:"idp_metadata"`
	CertPEM         string `json:"cert_pem"` // public cert only — private key never returned
	Enabled         bool   `json:"enabled"`
	JITProvisioning bool   `json:"jit_provisioning"`
}

// GetOrgSAMLConfig handles GET /api/v1/admin/org/saml-config.
func (h *Handler) GetOrgSAMLConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, err := h.service.repo.GetOrgSAMLConfigPublic(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org saml config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve SAML config",
			"code":  "ADMIN_SAML_ERROR",
		})
	}
	if cfg == nil {
		return c.JSON(http.StatusOK, GetOrgSAMLConfigResponse{OrgID: orgID, Enabled: false, JITProvisioning: true})
	}
	return c.JSON(http.StatusOK, GetOrgSAMLConfigResponse{
		OrgID:           cfg.OrgID,
		EntityID:        cfg.EntityID,
		ACSURL:          cfg.ACSURL,
		IDPMetadata:     cfg.IDPMetadata,
		CertPEM:         cfg.CertPEM,
		Enabled:         cfg.Enabled,
		JITProvisioning: cfg.JITProvisioning,
	})
}

// UpdateOrgSAMLConfigInput is the request body for PUT /api/v1/admin/org/saml-config.
type UpdateOrgSAMLConfigInput struct {
	EntityID        string `json:"entity_id"        validate:"required,url"`
	ACSURL          string `json:"acs_url"          validate:"required,url"`
	IDPMetadata     string `json:"idp_metadata"     validate:"required"`
	Enabled         bool   `json:"enabled"`
	JITProvisioning bool   `json:"jit_provisioning"`
}

// UpdateOrgSAMLConfig handles PUT /api/v1/admin/org/saml-config.
// If no cert/key exists, a new self-signed cert is generated automatically.
func (h *Handler) UpdateOrgSAMLConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgSAMLConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input", "code": "ADMIN_BAD_REQUEST"})
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "ADMIN_VALIDATION_ERROR"})
	}
	if err := h.service.repo.UpsertOrgSAMLConfig(c.Request().Context(), orgID, in.EntityID, in.ACSURL, in.IDPMetadata, in.Enabled, in.JITProvisioning); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org saml config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update SAML config", "code": "ADMIN_SAML_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// RegenerateSAMLCert handles POST /api/v1/admin/org/saml-config/regenerate-cert.
func (h *Handler) RegenerateSAMLCert(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	certPEM, err := h.service.repo.RegenerateSAMLCert(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("regenerate saml cert failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cert generation failed", "code": "ADMIN_SAML_CERT_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"cert_pem": certPEM, "status": "ok"})
}

// ─── S21-4: SCIM Token Management ────────────────────────────────────────────

// ListSCIMTokens handles GET /api/v1/admin/scim/tokens.
// Returns all tokens for the org.  Raw token values are never returned.
func (h *Handler) ListSCIMTokens(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	tokens, err := h.service.repo.ListSCIMTokens(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list scim tokens failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list SCIM tokens",
			"code":  "SCIM_TOKEN_LIST_ERROR",
		})
	}
	if tokens == nil {
		tokens = []SCIMToken{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": tokens})
}

// createSCIMTokenInput is the request body for POST /api/v1/admin/scim/tokens.
type createSCIMTokenInput struct {
	Name          string `json:"name"            validate:"required,min=1,max=128"`
	ExpiresInDays int    `json:"expires_in_days" validate:"min=0,max=3650"` // 0 = never expire
}

// CreateSCIMToken handles POST /api/v1/admin/scim/tokens.
// Generates a random 32-byte token, returns it ONCE in the response (plain text),
// and stores only the sha256 hex digest.
func (h *Handler) CreateSCIMToken(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input createSCIMTokenInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "SCIM_TOKEN_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "SCIM_TOKEN_VALIDATION_ERROR",
		})
	}

	// Generate a cryptographically random 32-byte token.
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		log.Error().Err(err).Msg("generate scim token entropy failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to generate token",
			"code":  "SCIM_TOKEN_ENTROPY_ERROR",
		})
	}
	rawToken := hex.EncodeToString(rawBytes) // 64-char hex string

	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	var expiresAt *time.Time
	if input.ExpiresInDays > 0 {
		t := time.Now().UTC().AddDate(0, 0, input.ExpiresInDays)
		expiresAt = &t
	}

	tok, err := h.service.repo.CreateSCIMToken(c.Request().Context(), orgID, input.Name, tokenHash, expiresAt)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create scim token failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create SCIM token",
			"code":  "SCIM_TOKEN_CREATE_ERROR",
		})
	}

	// Return the raw token exactly ONCE.  It will never be retrievable again.
	return c.JSON(http.StatusCreated, map[string]any{
		"id":         tok.ID,
		"name":       tok.Name,
		"token":      rawToken, // shown only once
		"created_at": tok.CreatedAt,
		"expires_at": tok.ExpiresAt,
	})
}

// RevokeSCIMToken handles DELETE /api/v1/admin/scim/tokens/:id.
func (h *Handler) RevokeSCIMToken(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	tokenID := c.Param("id")

	if err := h.service.repo.RevokeSCIMToken(c.Request().Context(), orgID, tokenID); err != nil {
		log.Error().Err(err).Str("token_id", tokenID).Msg("revoke scim token failed")
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "SCIM token not found or already revoked",
			"code":  "SCIM_TOKEN_NOT_FOUND",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "revoked"})
}

// ─── S105-2: OIDC/Casdoor Config ─────────────────────────────────────────────

// GetOrgOIDCConfig handles GET /api/v1/admin/org/oidc-config.
func (h *Handler) GetOrgOIDCConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, err := h.service.GetOIDCConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get oidc config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve OIDC config",
			"code":  "ADMIN_OIDC_ERROR",
		})
	}
	if cfg == nil {
		return c.JSON(http.StatusOK, map[string]any{"configured": false})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"configured":   true,
		"provider_url": cfg.ProviderURL,
		"client_id":    cfg.ClientID,
		"enabled":      cfg.Enabled,
		"updated_at":   cfg.UpdatedAt,
	})
}

// UpdateOrgOIDCConfigInput is the request body for PUT /api/v1/admin/org/oidc-config.
type UpdateOrgOIDCConfigInput struct {
	ProviderURL  string `json:"provider_url"  validate:"required,url"`
	ClientID     string `json:"client_id"     validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	Enabled      bool   `json:"enabled"`
}

// UpdateOrgOIDCConfig handles PUT /api/v1/admin/org/oidc-config.
func (h *Handler) UpdateOrgOIDCConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgOIDCConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input", "code": "ADMIN_BAD_REQUEST"})
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "ADMIN_VALIDATION_ERROR"})
	}
	if err := h.service.UpsertOIDCConfig(c.Request().Context(), orgID, OIDCConfigInput(in)); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update oidc config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update OIDC config", "code": "ADMIN_OIDC_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteOrgOIDCConfig handles DELETE /api/v1/admin/org/oidc-config (disables, doesn't delete).
func (h *Handler) DeleteOrgOIDCConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if err := h.service.DisableOIDCConfig(c.Request().Context(), orgID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("disable oidc config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to disable OIDC config", "code": "ADMIN_OIDC_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// ─── S105-3: SAML Metadata URL fetch ─────────────────────────────────────────

// FetchSAMLMetadataInput is the request body for POST /admin/org/saml-config/fetch-metadata.
type FetchSAMLMetadataInput struct {
	URL string `json:"url" validate:"required,url"`
}

// FetchSAMLMetadata handles POST /api/v1/admin/org/saml-config/fetch-metadata.
// Fetches IdP metadata XML from a URL (max 512 KB, 10s timeout).
func (h *Handler) FetchSAMLMetadata(c echo.Context) error {
	var in FetchSAMLMetadataInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input", "code": "ADMIN_BAD_REQUEST"})
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "code": "ADMIN_VALIDATION_ERROR"})
	}

	xml, err := fetchMetadataFromURL(c.Request().Context(), in.URL)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_SAML_FETCH_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"metadata": xml})
}
