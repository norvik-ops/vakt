package dataexport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes mounts the export endpoint on the provided group.
// g should already be a protected (auth-required) group — e.g. protected.Group("/export").
// version is the running Vakt version, recorded in the archive's meta.json.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool, version string) {
	g.GET("", ExportHandler(db, version))
	g.GET("/full", ExportHandler(db, version))
}
