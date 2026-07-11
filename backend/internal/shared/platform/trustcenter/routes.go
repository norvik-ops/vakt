package trustcenter

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts the public trust center route (no auth required).
func Register(e *echo.Echo, db *pgxpool.Pool) {
	h := NewHandler(db)
	e.GET("/trust/:slug", h.GetTrustCenter)
}

// RegisterAdmin mounts admin trust center routes under the provided authenticated group.
// The group should already have auth middleware applied.
//
// S121-B2 (R2): the function name promised Admin gating that was never applied,
// so any authenticated user (incl. Viewer) could edit the public trust-center
// page and publish/unpublish policies and certificates. Every mutating route is
// now gated to Admin. Reads stay open to authenticated users so the admin
// console can render its current state.
func RegisterAdmin(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	admin := auth.RequireRole("Admin")
	tc := g.Group("/trust-center")
	tc.GET("/settings", h.GetTrustCenterSettings)
	tc.PATCH("/settings", h.UpdateTrustCenterSettings, admin)
	tc.GET("/certificates", h.ListCertificates)
	tc.POST("/certificates", h.CreateCertificate, admin)
	tc.DELETE("/certificates/:id", h.DeleteCertificate, admin)
	tc.GET("/policies", h.ListPublishedPolicies)
	tc.POST("/policies/:policyId/publish", h.PublishPolicy, admin)
	tc.DELETE("/policies/:policyId/publish", h.UnpublishPolicy, admin)
}
