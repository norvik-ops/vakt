package dataexport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes mounts the export endpoint on the provided group.
// g should already be a protected (auth-required) group — e.g. protected.Group("/export").
// modulesEnabled is the VAKT_MODULES_ENABLED CSV so the export respects per-module
// activation (HR/Aware files only when their module is on — S89-2).
// version is the running Vakt version, recorded in the archive's meta.json.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, modulesEnabled, version string) {
	g.GET("", ExportHandler(db, modulesEnabled, version))
	g.GET("/full", ExportHandler(db, modulesEnabled, version))
}
