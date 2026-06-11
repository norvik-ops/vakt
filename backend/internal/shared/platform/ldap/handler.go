package ldap

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler exposes LDAP configuration and sync operations over HTTP.
type Handler struct {
	cfg Config
}

// NewHandler creates a new LDAP Handler with the provided configuration.
func NewHandler(cfg Config) *Handler {
	return &Handler{cfg: cfg}
}

// ldapConfigResponse is the public representation of the LDAP config.
// BindPass is intentionally omitted.
type ldapConfigResponse struct {
	URL         string `json:"url"`
	BindDN      string `json:"bind_dn"`
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	TLS         bool   `json:"tls"`
	Enabled     bool   `json:"enabled"`
}

// ldapConfigInput is the request body for PUT /settings/ldap.
type ldapConfigInput struct {
	URL         string `json:"url"`
	BindDN      string `json:"bind_dn"`
	BindPass    string `json:"bind_pass"`
	BaseDN      string `json:"base_dn"`
	UserFilter  string `json:"user_filter"`
	GroupFilter string `json:"group_filter"`
	TLS         bool   `json:"tls"`
}

// GetConfig handles GET /api/v1/settings/ldap.
// Returns the current LDAP configuration without the bind password.
func (h *Handler) GetConfig(c echo.Context) error {
	resp := ldapConfigResponse{
		URL:         h.cfg.URL,
		BindDN:      h.cfg.BindDN,
		BaseDN:      h.cfg.BaseDN,
		UserFilter:  h.cfg.UserFilter,
		GroupFilter: h.cfg.GroupFilter,
		TLS:         h.cfg.TLS,
		Enabled:     h.cfg.Enabled(),
	}
	return c.JSON(http.StatusOK, resp)
}

// UpdateConfig handles PUT /api/v1/settings/ldap.
// Accepts updated LDAP configuration. Config persistence is managed via env vars;
// this endpoint acknowledges the input and returns success.
func (h *Handler) UpdateConfig(c echo.Context) error {
	var input ldapConfigInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "LDAP_BAD_REQUEST",
		})
	}

	// Update the in-memory config for the lifetime of this request context.
	// Durable persistence is handled via env vars / restart cycle.
	h.cfg = Config(input)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "LDAP configuration updated. Restart the service to persist changes via environment variables.",
	})
}

// TestConnection handles POST /api/v1/settings/ldap/test.
// Attempts to connect and bind to the LDAP server, then returns the number of users found.
func (h *Handler) TestConnection(c echo.Context) error {
	if !h.cfg.Enabled() {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "LDAP is not configured (url, bind_dn, and base_dn are required)",
			"code":  "LDAP_NOT_CONFIGURED",
		})
	}

	ctx := c.Request().Context()
	syncer := NewSyncer(h.cfg)

	users, err := syncer.ListUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ldap connection test failed")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "LDAP connection failed — check server logs for details",
			"code":  "LDAP_CONNECTION_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":          true,
		"users_found": len(users),
	})
}

// Sync handles POST /api/v1/settings/ldap/sync.
// Triggers a directory sync and returns the list of discovered users.
func (h *Handler) Sync(c echo.Context) error {
	if !h.cfg.Enabled() {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "LDAP is not configured (url, bind_dn, and base_dn are required)",
			"code":  "LDAP_NOT_CONFIGURED",
		})
	}

	ctx := c.Request().Context()
	syncer := NewSyncer(h.cfg)

	users, err := syncer.ListUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ldap sync failed")
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": "LDAP sync failed — check server logs for details",
			"code":  "LDAP_SYNC_ERROR",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"synced": len(users),
		"users":  users,
	})
}
