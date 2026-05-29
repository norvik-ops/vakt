package evidence_auto

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers the auto-evidence inbox endpoints under the given
// Echo group (expected: the /vaktcomply protected group).
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.GET("/evidence/auto", h.ListAutoEvidence)
	g.POST("/evidence/auto/:id/assign", h.AssignAutoEvidence)
}
