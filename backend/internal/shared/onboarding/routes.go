package onboarding

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes mounts the onboarding endpoints onto g.
// The group must already be protected by auth middleware so that org_id is
// available in the echo.Context.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	g.GET("/status", GetStatus(db))
	g.POST("/dismiss", Dismiss(db))
	// S89-5: guided "first 30 days" path (7 data-derived steps).
	g.GET("/progress", GetProgressHandler(db))
}
