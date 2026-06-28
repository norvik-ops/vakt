package admin

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/matharnica/vakt/internal/auth"
	siemSvc "github.com/matharnica/vakt/internal/services/siem"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
	"github.com/matharnica/vakt/internal/shared/platform/features"
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
	admin.POST("/users", h.CreateUser) // S105-1: direct creation, no SMTP
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

	// Per-org AI model selection (S32-3 ADR-0024)
	admin.GET("/org/ai-settings", h.GetOrgAISettings)
	admin.PUT("/org/ai-settings", h.UpdateOrgAISettings)

	// Per-org SMTP configuration (DB-backed, password encrypted with master key)
	admin.GET("/org/smtp", h.GetOrgSMTPSettings)
	admin.PUT("/org/smtp", h.UpdateOrgSMTPSettings)

	// Per-org backup configuration (DB-backed, secrets encrypted with master key)
	admin.GET("/org/backup-config", h.GetOrgBackupConfig)
	admin.PUT("/org/backup-config", h.UpdateOrgBackupConfig)

	// Per-org LDAP/AD configuration (DB-backed, bind password encrypted with master key)
	admin.GET("/org/ldap", h.GetOrgLDAPConfig)
	admin.PUT("/org/ldap", h.UpdateOrgLDAPConfig)
	admin.POST("/org/ldap/test", h.TestOrgLDAPConnection)
	admin.POST("/org/ldap/sync", h.SyncOrgLDAP)

	// Pro: per-org IP allowlist + MFA-for-sensitive-calls (S21-5, S21-6)
	admin.GET("/org/security-ext", h.GetOrgSecurityExtensions)
	admin.PUT("/org/ip-allowlist", h.UpdateOrgIPAllowlist, features.Require(features.FeatureAPI))
	admin.PUT("/org/mfa-sensitive", h.UpdateOrgMFASensitive, features.Require(features.FeatureAPI))

	// Per-user module permissions (GET is Community; PUT requires Pro)
	admin.GET("/users/:user_id/permissions", h.Permissions.GetPermissions)
	admin.PUT("/users/:user_id/permissions", h.Permissions.UpdatePermissions, features.Require(features.FeatureGranularPermissions))

	// Security events dashboard + account unlock
	sec := NewSecurityHandler(db, rdb)
	admin.GET("/security-events", sec.GetSecurityEvents)
	admin.DELETE("/accounts/:email/unlock", sec.UnlockAccount)

	// Pro-gated SIEM config
	siemH := siemSvc.NewHandler(siemSvc.NewService(db))
	admin.GET("/org/siem", siemH.GetSIEMConfig, features.Require(features.FeatureSIEM))
	admin.PUT("/org/siem", siemH.UpdateSIEMConfig, features.Require(features.FeatureSIEM))
	admin.POST("/org/siem/test", siemH.TestForward, features.Require(features.FeatureSIEM))

	// SAML direct SP config — Pro feature (S105-4: moved from CE in v0.17.0)
	admin.GET("/org/saml-config", h.GetOrgSAMLConfig, features.Require(features.FeatureSAMLAuth))
	admin.PUT("/org/saml-config", h.UpdateOrgSAMLConfig, features.Require(features.FeatureSAMLAuth))
	admin.POST("/org/saml-config/regenerate-cert", h.RegenerateSAMLCert, features.Require(features.FeatureSAMLAuth))
	admin.POST("/org/saml-config/fetch-metadata", h.FetchSAMLMetadata, features.Require(features.FeatureSAMLAuth))

	// Pro: OIDC/Casdoor config in DB (S105-2)
	admin.GET("/org/oidc-config", h.GetOrgOIDCConfig, features.Require(features.FeatureSSO))
	admin.PUT("/org/oidc-config", h.UpdateOrgOIDCConfig, features.Require(features.FeatureSSO))
	admin.DELETE("/org/oidc-config", h.DeleteOrgOIDCConfig, features.Require(features.FeatureSSO))

	// Pro-gated SCIM token management (S21-4)
	admin.GET("/scim/tokens", h.ListSCIMTokens, features.Require(features.FeatureSCIMProvisioning))
	admin.POST("/scim/tokens", h.CreateSCIMToken, features.Require(features.FeatureSCIMProvisioning))
	admin.DELETE("/scim/tokens/:id", h.RevokeSCIMToken, features.Require(features.FeatureSCIMProvisioning))

}
