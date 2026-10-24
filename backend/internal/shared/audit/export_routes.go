package audit

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the audit export endpoint onto the given group.
// The group must already have auth middleware applied.
func RegisterExport(g *echo.Group, db *pgxpool.Pool) {
	h := NewExportHandler(db)
	g.GET("/export/audit-package", h.Export)
}
