package audit

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the audit export endpoint onto the given group.
// The group must already have auth middleware applied. scanOn says whether Vakt
// Scan is enabled; without it the package carries no findings (see GeneratePackage).
func RegisterExport(g *echo.Group, db *pgxpool.Pool, scanOn bool) {
	h := NewExportHandler(db)
	h.scanOn = scanOn
	g.GET("/export/audit-package", h.Export)
}
