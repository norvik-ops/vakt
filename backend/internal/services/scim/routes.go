package scim

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register mounts the SCIM 2.0 endpoint group under g.
// All routes are protected by the SCIM Bearer token middleware (not Paseto).
// The group must be an unauthenticated echo.Group (no Paseto middleware) because
// SCIM clients authenticate with their own Bearer token, not a user session.
// revoker is optional — when non-nil, deactivation calls immediately revoke sessions.
func Register(g *echo.Group, db *pgxpool.Pool, revoker ...SessionRevoker) {
	svc := NewService(db)
	if len(revoker) > 0 && revoker[0] != nil {
		svc.WithSessionRevoker(revoker[0])
	}
	h := NewHandler(svc)

	// All SCIM endpoints require FeatureSCIMProvisioning.
	// The feature gate is evaluated against the license that is attached to the
	// Echo instance (set in main.go). Because SCIM uses its own auth (no Paseto
	// context), we attach the feature check as a route-level middleware so that
	// the license context has already been populated by license.DBMiddleware.
	// Note: for SCIM the license is org-specific — it is loaded from the DB by
	// the SCIMAuthMiddleware (which sets scim_org_id), but the license context
	// is the global one set at startup.  Pro enforcement here uses the platform
	// license, which is the correct behaviour for a self-hosted single-tenant
	// deployment.
	// 5 req/s burst 10 per token — prevents credential stuffing and runaway IdP sync loops.
	scimLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5), Burst: 10, ExpiresIn: 5 * time.Minute},
	))

	scim := g.Group("",
		features.Require(features.FeatureSCIMProvisioning),
		SCIMAuthMiddleware(db),
		scimLimiter,
	)

	// ServiceProviderConfig — discovery endpoint, no auth required by spec but
	// we keep it inside the feature gate so CE installations get a 402.
	scim.GET("/ServiceProviderConfig", h.GetServiceProviderConfig)

	// Users
	scim.GET("/Users", h.ListUsers)
	scim.POST("/Users", h.CreateUser)
	scim.GET("/Users/:id", h.GetUser)
	scim.PUT("/Users/:id", h.ReplaceUser)
	scim.PATCH("/Users/:id", h.PatchUser)
	scim.DELETE("/Users/:id", h.DeleteUser)

	// Groups
	scim.GET("/Groups", h.ListGroups)
	scim.POST("/Groups", h.CreateGroup)
	scim.GET("/Groups/:id", h.GetGroup)
	scim.PUT("/Groups/:id", h.ReplaceGroup)
	scim.PATCH("/Groups/:id", h.PatchGroup)
	scim.DELETE("/Groups/:id", h.DeleteGroup)
}
