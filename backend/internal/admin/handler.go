// Package admin provides admin panel endpoints for audit logs, user management,
// and module status.
package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/platform/features"
	platformldap "github.com/matharnica/vakt/internal/shared/platform/ldap"
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
		log.Error().Err(err).Str("org_id", orgID).Str("email_redacted", logsafe.RedactEmail(input.Email)).Msg("invite user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to invite user",
			"code":  "ADMIN_INVITE_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "invitation created",
	})
}

// CreateUser handles POST /api/v1/admin/users.
// Creates a user directly (no email invite, no SMTP required). The user is
// immediately active. Admin sees the initial password; it cannot be retrieved again.
func (h *Handler) CreateUser(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	creatorID, _ := c.Get("user_id").(string)

	var input CreateUserInput
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

	result, err := h.service.CreateUser(c.Request().Context(), orgID, creatorID, input)
	if err != nil {
		if err.Error() == "create user: email already exists" {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "email already in use",
				"code":  "ADMIN_USER_EXISTS",
			})
		}
		log.Error().Err(err).Str("org_id", orgID).Str("email_redacted", logsafe.RedactEmail(input.Email)).Msg("create user failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create user",
			"code":  "ADMIN_CREATE_USER_ERROR",
		})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"user_id": result.UserID,
		"email":   input.Email,
		"role":    input.Role,
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
	ModelOverride       string `json:"model_override"`
	BaseURLOverride     string `json:"base_url_override"`
	WeeklyDigestEnabled bool   `json:"weekly_digest_enabled"`
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
	if err := h.service.repo.SetOrgAISettings(c.Request().Context(), orgID, in.ModelOverride, in.BaseURLOverride, in.WeeklyDigestEnabled); err != nil {
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

// GetOrgSMTPSettings handles GET /api/v1/admin/org/smtp.
// Returns the per-org SMTP configuration. The password is never exposed in plaintext;
// HasPass signals whether an encrypted password is currently stored.
func (h *Handler) GetOrgSMTPSettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	s, err := h.service.repo.GetOrgSMTPSettings(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org smtp settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve SMTP settings",
			"code":  "ADMIN_SMTP_SETTINGS_ERROR",
		})
	}
	return c.JSON(http.StatusOK, s)
}

// UpdateOrgSMTPSettingsInput is the request body for PUT /api/v1/admin/org/smtp.
type UpdateOrgSMTPSettingsInput struct {
	Host string `json:"host"`
	Port string `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"` // empty = keep existing password
	From string `json:"from"`
	TLS  bool   `json:"tls"`
}

// UpdateOrgSMTPSettings handles PUT /api/v1/admin/org/smtp.
// If Pass is non-empty it is encrypted with the master key and stored.
// If Pass is empty the existing encrypted password is kept unchanged.
func (h *Handler) UpdateOrgSMTPSettings(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgSMTPSettingsInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	var passEnc []byte
	if in.Pass != "" {
		if len(h.service.masterKey) == 0 {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
		}
		var encErr error
		passEnc, encErr = sharedcrypto.Encrypt(h.service.masterKey, []byte(in.Pass))
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("smtp password encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt SMTP password",
				"code":  "ADMIN_SMTP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgSMTPSettings(c.Request().Context(), orgID, in.Host, in.Port, in.User, in.From, in.TLS, passEnc); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org smtp settings failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update SMTP settings",
			"code":  "ADMIN_SMTP_SETTINGS_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ─── Backup Configuration (Migration 230) ────────────────────────────────────

// GetOrgBackupConfig handles GET /api/v1/admin/org/backup-config.
// Returns the per-org backup configuration. Encrypted secrets are never exposed;
// HasPassphrase and HasNotifyWebhook signal whether values are stored.
func (h *Handler) GetOrgBackupConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, _, _, err := h.service.repo.GetOrgBackupConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org backup config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve backup configuration",
			"code":  "ADMIN_BACKUP_CONFIG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateOrgBackupConfigInput is the request body for PUT /api/v1/admin/org/backup-config.
type UpdateOrgBackupConfigInput struct {
	Schedule      string `json:"schedule"`       // cron expression; "" = use env default
	RetentionDays int    `json:"retention_days"` // 0 = use env default
	Passphrase    string `json:"passphrase"`     // empty = keep existing
	NotifyWebhook string `json:"notify_webhook"` // empty = keep existing
	OffsiteCmd    string `json:"offsite_cmd"`
	NotifyCmd     string `json:"notify_cmd"`
}

// UpdateOrgBackupConfig handles PUT /api/v1/admin/org/backup-config.
// Non-empty Passphrase and NotifyWebhook are encrypted with the master key.
// Empty values leave existing encrypted data unchanged.
func (h *Handler) UpdateOrgBackupConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgBackupConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	encrypt := func(plaintext string) ([]byte, error) {
		if len(h.service.masterKey) == 0 {
			return nil, fmt.Errorf("master key not configured")
		}
		return sharedcrypto.Encrypt(h.service.masterKey, []byte(plaintext))
	}

	var passphraseEnc []byte
	if in.Passphrase != "" {
		var encErr error
		passphraseEnc, encErr = encrypt(in.Passphrase)
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("backup passphrase encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt backup passphrase",
				"code":  "ADMIN_BACKUP_ENCRYPT_ERROR",
			})
		}
	}

	var webhookEnc []byte
	if in.NotifyWebhook != "" {
		var encErr error
		webhookEnc, encErr = encrypt(in.NotifyWebhook)
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("backup notify webhook encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt backup notify webhook",
				"code":  "ADMIN_BACKUP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgBackupConfig(
		c.Request().Context(), orgID,
		in.Schedule, in.RetentionDays,
		passphraseEnc, webhookEnc,
		in.OffsiteCmd, in.NotifyCmd,
	); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org backup config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update backup configuration",
			"code":  "ADMIN_BACKUP_CONFIG_UPDATE_ERROR",
		})
	}
	// Audit log: shell commands execute in the backup container as operator.
	// Log hash+presence only — never the content (may contain credentials).
	if in.OffsiteCmd != "" || in.NotifyCmd != "" {
		h256 := func(s string) string {
			if s == "" {
				return ""
			}
			sum := sha256.Sum256([]byte(s))
			return hex.EncodeToString(sum[:8]) // first 8 bytes sufficient for change-detection
		}
		log.Warn().
			Str("org_id", orgID).
			Bool("offsite_cmd_set", in.OffsiteCmd != "").
			Bool("notify_cmd_set", in.NotifyCmd != "").
			Str("offsite_cmd_sha256_prefix", h256(in.OffsiteCmd)).
			Str("notify_cmd_sha256_prefix", h256(in.NotifyCmd)).
			Msg("backup shell commands updated by admin — review if unexpected")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// InternalBackupConfigResponse is the plaintext response for the backup script endpoint.
type InternalBackupConfigResponse struct {
	Schedule      string `json:"schedule"`
	RetentionDays int    `json:"retention_days"`
	Passphrase    string `json:"passphrase"`     // decrypted plaintext
	NotifyWebhook string `json:"notify_webhook"` // decrypted plaintext
	OffsiteCmd    string `json:"offsite_cmd"`
	NotifyCmd     string `json:"notify_cmd"`
}

// GetInternalBackupConfig handles GET /api/v1/internal/backup-config.
// Auth: "Authorization: Bearer <VAKT_SECRET_KEY>" (hex-encoded master key).
// Returns plaintext backup configuration so the backup script can use it directly.
// This endpoint has no JWT middleware — it uses the master key as the Bearer token.
func (h *Handler) GetInternalBackupConfig(c echo.Context) error {
	// Validate Bearer token against hex-encoded master key.
	authHeader := c.Request().Header.Get("Authorization")
	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "UNAUTHORIZED",
		})
	}
	token := authHeader[len(prefix):]
	expected := hex.EncodeToString(h.service.masterKey)
	if len(h.service.masterKey) == 0 || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "UNAUTHORIZED",
		})
	}

	ctx := c.Request().Context()

	// Single-tenant: query the first org.
	var orgID string
	if err := h.service.db.QueryRow(ctx, `SELECT id::text FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		log.Error().Err(err).Msg("internal backup config: no org found")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "no organization found",
			"code":  "INTERNAL_BACKUP_NO_ORG",
		})
	}

	cfg, passphraseEnc, webhookEnc, err := h.service.repo.GetOrgBackupConfig(ctx, orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("internal backup config: get config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve backup configuration",
			"code":  "INTERNAL_BACKUP_CONFIG_ERROR",
		})
	}

	resp := InternalBackupConfigResponse{
		Schedule:      cfg.Schedule,
		RetentionDays: cfg.RetentionDays,
		OffsiteCmd:    cfg.OffsiteCmd,
		NotifyCmd:     cfg.NotifyCmd,
	}

	if len(passphraseEnc) > 0 {
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, passphraseEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("internal backup config: passphrase decrypt failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt backup passphrase",
				"code":  "INTERNAL_BACKUP_DECRYPT_ERROR",
			})
		}
		resp.Passphrase = string(plain)
	}

	if len(webhookEnc) > 0 {
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, webhookEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("internal backup config: notify webhook decrypt failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt backup notify webhook",
				"code":  "INTERNAL_BACKUP_DECRYPT_ERROR",
			})
		}
		resp.NotifyWebhook = string(plain)
	}

	return c.JSON(http.StatusOK, resp)
}

// ─── Migration 231: LDAP/AD Configuration ────────────────────────────────────

// GetOrgLDAPConfig handles GET /api/v1/admin/org/ldap.
// Returns the per-org LDAP configuration. The bind password is never exposed
// in plaintext; HasBindPass signals whether an encrypted password is stored.
func (h *Handler) GetOrgLDAPConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	cfg, _, err := h.service.repo.GetOrgLDAPConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get org ldap config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_ERROR",
		})
	}
	return c.JSON(http.StatusOK, cfg)
}

// UpdateOrgLDAPConfigInput is the request body for PUT /api/v1/admin/org/ldap.
type UpdateOrgLDAPConfigInput struct {
	URL         string `json:"url"`
	BindDN      string `json:"bind_dn"`
	BindPass    string `json:"bind_pass"` // empty = keep existing password
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	TLS         bool   `json:"tls"`
}

// UpdateOrgLDAPConfig handles PUT /api/v1/admin/org/ldap.
// If BindPass is non-empty it is encrypted with the master key and stored.
// If BindPass is empty the existing encrypted password is kept unchanged.
func (h *Handler) UpdateOrgLDAPConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	var in UpdateOrgLDAPConfigInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid input",
			"code":  "ADMIN_BAD_REQUEST",
		})
	}

	var bindPassEnc []byte
	if in.BindPass != "" {
		if len(h.service.masterKey) == 0 {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
		}
		var encErr error
		bindPassEnc, encErr = sharedcrypto.Encrypt(h.service.masterKey, []byte(in.BindPass))
		if encErr != nil {
			log.Error().Err(encErr).Str("org_id", orgID).Msg("ldap bind password encryption failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to encrypt LDAP bind password",
				"code":  "ADMIN_LDAP_ENCRYPT_ERROR",
			})
		}
	}

	if err := h.service.repo.SetOrgLDAPConfig(
		c.Request().Context(), orgID,
		in.URL, in.BindDN, in.BaseDN, in.UserFilter, in.GroupFilter,
		in.TLS, bindPassEnc,
	); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update org ldap config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_UPDATE_ERROR",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// buildLDAPSyncer retrieves the org's LDAP config from the DB, decrypts the
// bind password, and returns a ready-to-use Syncer. Returns a descriptive
// HTTP error response via c.JSON on failure (callers must return immediately).
func (h *Handler) buildLDAPSyncer(c echo.Context, orgID string) (*platformldap.Syncer, error) {
	cfg, bindPassEnc, err := h.service.repo.GetOrgLDAPConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap test: get config failed")
		_ = c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to retrieve LDAP configuration",
			"code":  "ADMIN_LDAP_CONFIG_ERROR",
		})
		return nil, err
	}

	var bindPass string
	if len(bindPassEnc) > 0 {
		if len(h.service.masterKey) == 0 {
			_ = c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "master key not configured",
				"code":  "ADMIN_NO_MASTER_KEY",
			})
			return nil, fmt.Errorf("master key not configured")
		}
		plain, decErr := sharedcrypto.Decrypt(h.service.masterKey, bindPassEnc)
		if decErr != nil {
			log.Error().Err(decErr).Str("org_id", orgID).Msg("ldap test: bind password decrypt failed")
			_ = c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to decrypt LDAP bind password",
				"code":  "ADMIN_LDAP_DECRYPT_ERROR",
			})
			return nil, decErr
		}
		bindPass = string(plain)
	}

	ldapCfg := platformldap.Config{
		URL:         cfg.URL,
		BindDN:      cfg.BindDN,
		BindPass:    bindPass,
		BaseDN:      cfg.BaseDN,
		UserFilter:  cfg.UserFilter,
		GroupFilter: cfg.GroupFilter,
		TLS:         cfg.TLS,
	}
	return platformldap.NewSyncer(ldapCfg), nil
}

// TestOrgLDAPConnection handles POST /api/v1/admin/org/ldap/test.
// Connects to the configured LDAP server, lists users, and returns the count.
func (h *Handler) TestOrgLDAPConnection(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	syncer, err := h.buildLDAPSyncer(c, orgID)
	if err != nil {
		return nil // response already written by buildLDAPSyncer
	}

	users, err := syncer.ListUsers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap test: list users failed")
		return c.JSON(http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
			"code":  "ADMIN_LDAP_TEST_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":          true,
		"users_found": len(users),
	})
}

// SyncOrgLDAP handles POST /api/v1/admin/org/ldap/sync.
// Connects to the configured LDAP server, retrieves all users, and returns them.
func (h *Handler) SyncOrgLDAP(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)

	syncer, err := h.buildLDAPSyncer(c, orgID)
	if err != nil {
		return nil // response already written by buildLDAPSyncer
	}

	users, err := syncer.ListUsers(c.Request().Context())
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ldap sync: list users failed")
		return c.JSON(http.StatusBadGateway, map[string]any{
			"error": err.Error(),
			"code":  "ADMIN_LDAP_SYNC_FAILED",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"synced": len(users),
		"users":  users,
	})
}
