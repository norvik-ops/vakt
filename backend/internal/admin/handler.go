// Package admin provides admin panel endpoints for audit logs, user management,
// and module status.
package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Handler holds HTTP handler methods for admin endpoints.
type Handler struct {
	service     *Service
	validate    *validator.Validate
	Permissions *PermissionsHandler
}

// NewHandler constructs an admin Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:     service,
		validate:    validator.New(),
		Permissions: NewPermissionsHandler(service.db),
	}
}

// ListAuditLogs handles GET /api/v1/admin/audit-logs.
// Supports ?page=1&limit=25&user_id=&action=&resource_type= and ?format=csv.
func (h *Handler) ListAuditLogs(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	userFilter := c.QueryParam("user_id")
	actionFilter := c.QueryParam("action")
	resourceFilter := c.QueryParam("resource_type")

	logs, total, err := h.service.ListAuditLogs(
		c.Request().Context(), orgID, page, limit, userFilter, actionFilter, resourceFilter,
	)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list audit logs failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve audit logs",
			"code":  "ADMIN_AUDIT_ERROR",
		})
	}

	// CSV export
	if c.QueryParam("format") == "csv" {
		c.Response().Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
		c.Response().Header().Set("Content-Type", "text/csv")
		w := csv.NewWriter(c.Response().Writer)
		if err := w.Write([]string{
			"id", "org_id", "user_id", "action", "resource_type",
			"resource_id", "ip_address", "timestamp",
		}); err != nil {
			return err
		}
		for _, l := range logs {
			row := []string{
				l.ID, l.OrgID,
				derefString(l.UserID),
				l.Action, l.ResourceType,
				derefString(l.ResourceID),
				derefString(l.IPAddress),
				l.Timestamp.String(),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ListUsers handles GET /api/v1/admin/users.
func (h *Handler) ListUsers(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	members, err := h.service.ListUsers(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list users failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve users",
			"code":  "ADMIN_USERS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": members,
	})
}

// InviteUser handles POST /api/v1/admin/users/invite.
func (h *Handler) InviteUser(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	inviterID, _ := c.Get("user_id").(string)

	var input InviteInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_VALIDATION_ERROR",
		})
	}

	if err := h.service.InviteUser(c.Request().Context(), orgID, inviterID, input); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Str("email", input.Email).Msg("invite user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to invite user",
			"code":  "ADMIN_INVITE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "invitation created",
	})
}

// UpdateUserRole handles PATCH /api/v1/admin/users/:id/role.
func (h *Handler) UpdateUserRole(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	targetUserID := c.Param("id")

	var input RoleUpdateInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_VALIDATION_ERROR",
		})
	}

	if err := h.service.UpdateUserRole(c.Request().Context(), orgID, targetUserID, input); err != nil {
		if err.Error() == "user not found in org" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "user not found",
				"code":  "ADMIN_USER_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("target_user_id", targetUserID).Msg("update user role failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update role",
			"code":  "ADMIN_ROLE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "role updated",
	})
}

// ListModules handles GET /api/v1/admin/modules.
func (h *Handler) ListModules(c echo.Context) error {
	modules := h.service.ListModules()
	return c.JSON(http.StatusOK, map[string]any{
		"data": modules,
	})
}

// ListNotificationChannels handles GET /api/v1/admin/notifications/channels.
func (h *Handler) ListNotificationChannels(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	channels, err := h.service.ListNotificationChannels(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("list notification channels failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve notification channels",
			"code":  "ADMIN_NOTIFY_CHANNELS_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": channels,
	})
}

// CreateNotificationChannel handles POST /api/v1/admin/notifications/channels.
func (h *Handler) CreateNotificationChannel(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var input notify.CreateChannelInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_VALIDATION_ERROR",
		})
	}

	ch, err := h.service.CreateNotificationChannel(c.Request().Context(), orgID, input)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("create notification channel failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create notification channel",
			"code":  "ADMIN_NOTIFY_CREATE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, ch)
}

// DeleteNotificationChannel handles DELETE /api/v1/admin/notifications/channels/:id.
func (h *Handler) DeleteNotificationChannel(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	channelID := c.Param("id")

	if err := h.service.DeleteNotificationChannel(c.Request().Context(), orgID, channelID); err != nil {
		if err.Error() == "notification channel not found" {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "notification channel not found",
				"code":  "ADMIN_NOTIFY_NOT_FOUND",
			})
		}
		log.Error().Err(err).Str("channel_id", channelID).Msg("delete notification channel failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete notification channel",
			"code":  "ADMIN_NOTIFY_DELETE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "notification channel deleted",
	})
}

// CreateManagedOrg handles POST /api/v1/admin/organizations.
// GetCurrentOrg handles GET /api/v1/admin/org.
// Returns the caller's own organisation record, including trust center settings.
func (h *Handler) GetCurrentOrg(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	org, err := h.service.repo.GetCurrentOrg(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get current org failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve organization",
			"code":  "ADMIN_ORG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"data": org})
}

// UpdateTrustCenter handles PUT /api/v1/admin/trust-center.
type UpdateTrustCenterInput struct {
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	Contact     string `json:"contact"`
}

func (h *Handler) UpdateTrustCenter(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateTrustCenterInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	if err := h.service.repo.UpdateOrgTrustCenter(c.Request().Context(), orgID, in.Enabled, in.Description, in.Contact); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update trust center failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
			"code":  "ADMIN_TRUST_CENTER_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetOrgSecurity handles GET /api/v1/admin/org/security.
// Returns the organisation's security policy settings (e.g. require_mfa).
func (h *Handler) GetOrgSecurity(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	sec, err := h.service.repo.GetOrgSecurity(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org security failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve org security settings",
			"code":  "ADMIN_ORG_SECURITY_ERROR",
		})
	}
	return c.JSON(http.StatusOK, sec)
}

// UpdateOrgSecurityInput is the request body for PUT /api/v1/admin/org/security.
type UpdateOrgSecurityInput struct {
	RequireMFA bool `json:"require_mfa"`
}

// UpdateOrgSecurity handles PUT /api/v1/admin/org/security.
// Allows admins to toggle org-wide MFA enforcement.
func (h *Handler) UpdateOrgSecurity(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	var in UpdateOrgSecurityInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	if err := h.service.repo.SetOrgRequireMFA(c.Request().Context(), orgID, in.RequireMFA); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org security failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update org security settings",
			"code":  "ADMIN_ORG_SECURITY_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetOrgAISettings handles GET /api/v1/admin/org/ai-settings.
// Returns the per-org AI model configuration (S32-3).
func (h *Handler) GetOrgAISettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	s, err := h.service.repo.GetOrgAISettings(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org ai settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve AI settings",
			"code":  "ADMIN_AI_SETTINGS_ERROR",
		})
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateOrgAISettingsInput is the request body for PUT /api/v1/admin/org/ai-settings.
type UpdateOrgAISettingsInput struct {
	ModelOverride   string `json:"model_override"`
	BaseURLOverride string `json:"base_url_override"`
}

// UpdateOrgAISettings handles PUT /api/v1/admin/org/ai-settings.
// base_url_override is only persisted when the org has a Pro license
// (FeatureAIAdvisor); CE orgs may only change the model name.
func (h *Handler) UpdateOrgAISettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgAISettingsInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}
	// CE orgs cannot set a custom base URL.
	if in.BaseURLOverride != "" && !features.IsEnabled(c, features.FeatureAIAdvisor) {
		in.BaseURLOverride = ""
	}
	if err := h.service.repo.SetOrgAISettings(c.Request().Context(), orgID, in.ModelOverride, in.BaseURLOverride); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org ai settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update AI settings",
			"code":  "ADMIN_AI_SETTINGS_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// derefString safely dereferences a string pointer for CSV output.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetOrgSecurityExtensions handles GET /api/v1/admin/org/security-ext (S21-5, S21-6).
func (h *Handler) GetOrgSecurityExtensions(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	s, err := h.service.repo.GetOrgSecurityExtensions(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org security ext failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve security settings",
			"code":  "ADMIN_SEC_EXT_ERROR",
		})
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateOrgIPAllowlistInput is the request body for PUT /api/v1/admin/org/ip-allowlist.
type UpdateOrgIPAllowlistInput struct {
	AllowList string `json:"admin_ip_allowlist"`
}

// UpdateOrgIPAllowlist handles PUT /api/v1/admin/org/ip-allowlist.
func (h *Handler) UpdateOrgIPAllowlist(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgIPAllowlistInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input", "code": "ADMIN_BAD_REQUEST"})
	}
	if err := h.service.repo.SetOrgIPAllowlist(c.Request().Context(), orgID, in.AllowList); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org ip allowlist failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update", "code": "ADMIN_IP_ALLOWLIST_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UpdateOrgMFASensitiveInput is the request body for PUT /api/v1/admin/org/mfa-sensitive.
type UpdateOrgMFASensitiveInput struct {
	RequireMFA bool `json:"require_mfa_sensitive_calls"`
}

// UpdateOrgMFASensitive handles PUT /api/v1/admin/org/mfa-sensitive.
func (h *Handler) UpdateOrgMFASensitive(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgMFASensitiveInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input", "code": "ADMIN_BAD_REQUEST"})
	}
	if err := h.service.repo.SetOrgRequireMFASensitiveCalls(c.Request().Context(), orgID, in.RequireMFA); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org mfa sensitive failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update", "code": "ADMIN_MFA_SENSITIVE_ERROR"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ─── S21-1: SAML Direct SP Config ────────────────────────────────────────────

// GetOrgSAMLConfigResponse is the public view of OrgSAMLConfig (key PEM omitted).
type GetOrgSAMLConfigResponse struct {
	OrgID       string `json:"org_id"`
	EntityID    string `json:"entity_id"`
	ACSURL      string `json:"acs_url"`
	IDPMetadata string `json:"idp_metadata"`
	CertPEM     string `json:"cert_pem"`   // public cert only — private key never returned
	Enabled     bool   `json:"enabled"`
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
		return c.JSON(http.StatusOK, GetOrgSAMLConfigResponse{OrgID: orgID, Enabled: false})
	}
	return c.JSON(http.StatusOK, GetOrgSAMLConfigResponse{
		OrgID:       cfg.OrgID,
		EntityID:    cfg.EntityID,
		ACSURL:      cfg.ACSURL,
		IDPMetadata: cfg.IDPMetadata,
		CertPEM:     cfg.CertPEM,
		Enabled:     cfg.Enabled,
	})
}

// UpdateOrgSAMLConfigInput is the request body for PUT /api/v1/admin/org/saml-config.
type UpdateOrgSAMLConfigInput struct {
	EntityID    string `json:"entity_id"    validate:"required,url"`
	ACSURL      string `json:"acs_url"      validate:"required,url"`
	IDPMetadata string `json:"idp_metadata" validate:"required"`
	Enabled     bool   `json:"enabled"`
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
	if err := h.service.repo.UpsertOrgSAMLConfig(c.Request().Context(), orgID, in.EntityID, in.ACSURL, in.IDPMetadata, in.Enabled); err != nil {
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
	Name string `json:"name" validate:"required,min=1,max=128"`
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

	tok, err := h.service.repo.CreateSCIMToken(c.Request().Context(), orgID, input.Name, tokenHash)
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
