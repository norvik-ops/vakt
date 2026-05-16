package auditexport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the audit export endpoint onto the given group.
// The group must already have auth middleware applied.
func Register(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.GET("/export/audit-package", h.Export)
}
