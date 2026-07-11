package evidence_auto

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// RegisterRoutes registers the auto-evidence inbox endpoints under the given
// Echo group (expected: the /vaktcomply protected group).
//
// S121-B4 (R4): assigning auto-collected evidence to a control mutates
// compliance-integrity state, but the assign route had no role gate — a Viewer
// or Analyst reached the handler. It is now gated to Admin. Listing the inbox
// stays open to authenticated users.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.GET("/evidence/auto", h.ListAutoEvidence)
	g.POST("/evidence/auto/:id/assign", h.AssignAutoEvidence, auth.RequireRole("Admin"))
}
