package dataexport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes mounts the export endpoint on the provided group.
// g should already be a protected (auth-required) group — e.g. protected.Group("/export").
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	g.GET("", ExportHandler(db))
	g.GET("/full", ExportHandler(db))
}
