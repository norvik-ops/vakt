package trustcenter

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts the public trust center data route (no auth required) under the
// /api/v1 group.
//
// S131-D4 (R-H13/D18-06): the route used to sit on the root echo at "/trust/:slug".
// Caddy only proxies /api/* and /health to the API — everything else falls to the
// SPA catch-all — so the root route was UNREACHABLE from any browser behind Caddy,
// and the public Trust Center page always showed "not found" despite live data. The
// frontend already fetches /api/v1/trust/:slug (TrustPage.tsx); mounting here makes
// both ends agree AND lets Caddy route it. The human-facing /trust/:slug URL stays
// an SPA route (Caddy → frontend), which renders TrustPage and then fetches this.
func Register(api *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	api.GET("/trust/:slug", h.GetTrustCenter)
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
