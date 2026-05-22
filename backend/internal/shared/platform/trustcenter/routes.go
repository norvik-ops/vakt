package trustcenter

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the public trust center route (no auth required).
func Register(e *echo.Echo, db *pgxpool.Pool) {
	h := NewHandler(db)
	e.GET("/trust/:slug", h.GetTrustCenter)
}

// RegisterAdmin mounts admin trust center routes under the provided authenticated group.
// The group should already have auth middleware applied.
func RegisterAdmin(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	tc := g.Group("/trust-center")
	tc.GET("/settings", h.GetTrustCenterSettings)
	tc.PATCH("/settings", h.UpdateTrustCenterSettings)
	tc.GET("/certificates", h.ListCertificates)
	tc.POST("/certificates", h.CreateCertificate)
	tc.DELETE("/certificates/:id", h.DeleteCertificate)
	tc.GET("/policies", h.ListPublishedPolicies)
	tc.POST("/policies/:policyId/publish", h.PublishPolicy)
	tc.DELETE("/policies/:policyId/publish", h.UnpublishPolicy)
}
