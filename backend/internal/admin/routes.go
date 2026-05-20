package admin

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/license"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// Register mounts admin routes under g.  All routes require the "Admin" role.
func Register(g *echo.Group, h *Handler, health *HealthHandler, db *pgxpool.Pool, rdb *redis.Client) {
	admin := g.Group("/admin", auth.RequireRole("Admin"), sharedmw.IPAllowlist())
	admin.GET("/health", health.HandleHealth)
	admin.GET("/audit-logs", h.ListAuditLogs)

	// SIEM export endpoints
	siem := NewSIEMHandler(db)
	admin.GET("/audit-log/export.cef", siem.ExportCEF)
	admin.GET("/audit-log/export.syslog", siem.ExportSyslog)

	admin.GET("/users", h.ListUsers)
	admin.POST("/users/invite", h.InviteUser)
	admin.PATCH("/users/:id/role", h.UpdateUserRole)
	admin.GET("/modules", h.ListModules)

	// Notification channel management
	admin.GET("/notifications/channels", h.ListNotificationChannels)
	admin.POST("/notifications/channels", h.CreateNotificationChannel)
	admin.DELETE("/notifications/channels/:id", h.DeleteNotificationChannel)

	// Current org info + trust center settings
	admin.GET("/org", h.GetCurrentOrg)
	admin.PUT("/trust-center", h.UpdateTrustCenter)

	// Org security policy (MFA enforcement, etc.)
	admin.GET("/org/security", h.GetOrgSecurity)
	admin.PUT("/org/security", h.UpdateOrgSecurity)

	// Per-user module permissions (GET is Community; PUT requires Pro)
	admin.GET("/users/:user_id/permissions", h.Permissions.GetPermissions)
	admin.PUT("/users/:user_id/permissions", h.Permissions.UpdatePermissions, license.Require(license.FeatureGranularPermissions))

	// Security events dashboard + account unlock
	sec := NewSecurityHandler(db, rdb)
	admin.GET("/security-events", sec.GetSecurityEvents)
	admin.DELETE("/accounts/:email/unlock", sec.UnlockAccount)

}

// RegisterStaging mounts staging-only routes. Call only when VAKT_STAGING=true.
func RegisterStaging(g *echo.Group, h *StagingHandler) {
	staging := g.Group("/admin/staging", auth.RequireRole("Admin"))
	staging.GET("/info", h.GetStagingInfo)
	staging.POST("/promote", h.Promote)
}
