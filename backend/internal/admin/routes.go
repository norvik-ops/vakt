package admin

import (
	"github.com/labstack/echo/v4"

	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/license"
)

// Register mounts admin routes under g.  All routes require the "Admin" role.
func Register(g *echo.Group, h *Handler) {
	admin := g.Group("/admin", auth.RequireRole("Admin"))
	admin.GET("/audit-logs", h.ListAuditLogs)
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

	// Per-user module permissions (GET is Community; PUT requires Pro)
	admin.GET("/users/:user_id/permissions", h.Permissions.GetPermissions)
	admin.PUT("/users/:user_id/permissions", h.Permissions.UpdatePermissions, license.Require(license.FeatureGranularPermissions))

	// MSP management (caller must be Admin of the parent MSP org)
	msp := admin.Group("/organizations")
	msp.POST("", h.CreateManagedOrg)
	msp.GET("", h.ListManagedOrgs)
	msp.DELETE("/:id", h.DeleteManagedOrg)
	msp.GET("/:id/branding", h.GetOrgBranding)
	msp.PUT("/:id/branding", h.UpdateOrgBranding)
}

// RegisterStaging mounts staging-only routes. Call only when VAKT_STAGING=true.
func RegisterStaging(g *echo.Group, h *StagingHandler) {
	staging := g.Group("/admin/staging", auth.RequireRole("Admin"))
	staging.GET("/info", h.GetStagingInfo)
	staging.POST("/promote", h.Promote)
}
