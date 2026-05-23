package siem

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler handles the SIEM config endpoints under /api/v1/admin/org/siem.
type Handler struct {
	svc *Service
}

// NewHandler constructs a SIEM Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// siemConfigResponse is the GET response — token is masked.
type siemConfigResponse struct {
	Enabled  bool   `json:"enabled"`
	Adapter  string `json:"adapter"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"` // "***" if set, "" if not
}

// updateSIEMConfigRequest is the PUT body.
type updateSIEMConfigRequest struct {
	Enabled  bool   `json:"enabled"`
	Adapter  string `json:"adapter"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"` // empty = keep existing
}

// GetSIEMConfig handles GET /api/v1/admin/org/siem.
func (h *Handler) GetSIEMConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_ORG",
		})
	}

	cfg, err := h.svc.GetOrgConfig(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("get siem config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to load SIEM config",
			"code":  "SIEM_CONFIG_ERROR",
		})
	}

	// Mask token — never return the real value.
	maskedToken := ""
	if cfg.Token != "" {
		maskedToken = "***"
	}

	return c.JSON(http.StatusOK, siemConfigResponse{
		Enabled:  cfg.Enabled,
		Adapter:  cfg.Adapter,
		Endpoint: cfg.Endpoint,
		Token:    maskedToken,
	})
}

// UpdateSIEMConfig handles PUT /api/v1/admin/org/siem.
func (h *Handler) UpdateSIEMConfig(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_ORG",
		})
	}

	var req updateSIEMConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "SIEM_INVALID_BODY",
		})
	}

	if err := h.svc.SetOrgConfig(c.Request().Context(), orgID, OrgSIEMConfig(req)); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("update siem config failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update SIEM config",
			"code":  "SIEM_UPDATE_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// TestForward handles POST /api/v1/admin/org/siem/test.
func (h *Handler) TestForward(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "AUTH_MISSING_ORG",
		})
	}

	if err := h.svc.TestForward(c.Request().Context(), orgID); err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("siem test forward failed")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": err.Error(),
			"code":  "SIEM_TEST_FAILED",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
