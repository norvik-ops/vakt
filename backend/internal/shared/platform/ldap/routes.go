package ldap

import (
	"github.com/labstack/echo/v4"
)

// Register wires LDAP settings routes under the provided echo.Group.
// All routes require the Admin role via the provided authMiddleware.
//
// Routes registered:
//
//	GET  /settings/ldap        — return current LDAP config (no bind password)
//	PUT  /settings/ldap        — update LDAP config
//	POST /settings/ldap/test   — test LDAP connection
//	POST /settings/ldap/sync   — trigger a user sync
func Register(g *echo.Group, cfg Config, authMiddleware echo.MiddlewareFunc) {
	h := NewHandler(cfg)

	ldapGroup := g.Group("/settings/ldap", authMiddleware)
	ldapGroup.GET("", h.GetConfig)
	ldapGroup.PUT("", h.UpdateConfig)
	ldapGroup.POST("/test", h.TestConnection)
	ldapGroup.POST("/sync", h.Sync)
}
