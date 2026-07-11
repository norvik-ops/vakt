package ldap

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register wires LDAP settings routes under the provided echo.Group.
// The caller passes the `protected` group (auth/CSRF/MFA already applied).
//
// S121-B5 (R7): the doc comment promised "All routes require the Admin role"
// but only an auth middleware was applied — any authenticated user (incl. Viewer)
// from any org could read the directory config, trigger syncs, and mutate the
// shared config. Every route is now gated to Admin. The LDAP config is a
// process-global env-var setting; see UpdateConfig for the cross-org shared-state
// fix.
//
// Routes registered:
//
//	GET  /settings/ldap        — return current LDAP config (no bind password)
//	PUT  /settings/ldap        — update LDAP config
//	POST /settings/ldap/test   — test LDAP connection
//	POST /settings/ldap/sync   — trigger a user sync
func Register(g *echo.Group, cfg Config, authMiddleware echo.MiddlewareFunc) {
	h := NewHandler(cfg)
	admin := auth.RequireRole("Admin")

	ldapGroup := g.Group("/settings/ldap", authMiddleware, admin)
	ldapGroup.GET("", h.GetConfig)
	ldapGroup.PUT("", h.UpdateConfig)
	ldapGroup.POST("/test", h.TestConnection)
	ldapGroup.POST("/sync", h.Sync)
}
