package demo

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// BlockedRoutes are mutating operations disabled in demo mode.
var BlockedRoutes = []BlockedRoute{
	// User & org management
	{Method: "POST", Prefix: "/api/v1/admin/users"},
	{Method: "DELETE", Prefix: "/api/v1/admin/users"},
	{Method: "PUT", Prefix: "/api/v1/admin/users"},
	{Method: "POST", Prefix: "/api/v1/admin/orgs"},
	{Method: "DELETE", Prefix: "/api/v1/admin/orgs"},
	// Password & 2FA
	{Method: "PUT", Prefix: "/api/v1/auth/password"},
	{Method: "POST", Prefix: "/api/v1/auth/totp"},
	{Method: "DELETE", Prefix: "/api/v1/auth/totp"},
	// Infrastructure settings
	{Method: "PUT", Prefix: "/api/v1/settings/smtp"},
	{Method: "POST", Prefix: "/api/v1/settings/smtp"},
	{Method: "PUT", Prefix: "/api/v1/settings/ldap"},
	{Method: "POST", Prefix: "/api/v1/settings/ldap"},
	{Method: "PUT", Prefix: "/api/v1/settings/branding"},
	{Method: "POST", Prefix: "/api/v1/settings/branding"},
	// Alerting
	{Method: "PUT", Prefix: "/api/v1/alerting/webhooks"},
	{Method: "POST", Prefix: "/api/v1/alerting/webhooks"},
	{Method: "DELETE", Prefix: "/api/v1/alerting/webhooks"},
	// SecVault — no creating/deleting real secrets
	{Method: "POST", Prefix: "/api/v1/vaktvault/secrets"},
	{Method: "DELETE", Prefix: "/api/v1/vaktvault/secrets"},
	{Method: "PUT", Prefix: "/api/v1/vaktvault/secrets"},
	// Retention & score config
	{Method: "PUT", Prefix: "/api/v1/retention"},
	{Method: "PUT", Prefix: "/api/v1/vaktcomply/score-config"},
}

type BlockedRoute struct {
	Method string
	Prefix string
}

// Guard returns Echo middleware that blocks mutating operations when demoMode is true.
func Guard(demoMode bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !demoMode {
				return next(c)
			}
			path := c.Request().URL.Path
			method := c.Request().Method
			for _, r := range BlockedRoutes {
				if method == r.Method && strings.HasPrefix(path, r.Prefix) {
					return c.JSON(http.StatusForbidden, map[string]string{
						"error": "Diese Funktion ist in der Demo-Umgebung nicht verfügbar.",
						"code":  "DEMO_RESTRICTED",
					})
				}
			}
			return next(c)
		}
	}
}
