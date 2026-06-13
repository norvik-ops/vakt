// Package retention registers HTTP routes for the data-retention configuration API.
package retention

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts retention-config routes under the provided Echo group.
// The caller passes an /api/v1-rooted group; routes are placed under /retention.
// auth is the Paseto middleware used for all protected routes.
func Register(g *echo.Group, db *pgxpool.Pool, auth echo.MiddlewareFunc) {
	h := NewHandler(db)
	r := g.Group("/retention", auth)
	r.GET("/config", h.GetConfig)
	r.PUT("/config", h.UpdateConfig)
}
