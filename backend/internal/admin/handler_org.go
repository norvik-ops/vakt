package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/platform/features"
)

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
