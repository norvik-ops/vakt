package onboarding

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// RegisterRoutes mounts the onboarding endpoints onto g.
// The group must already be protected by auth middleware so that org_id is
// available in the echo.Context.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	g.GET("/status", GetStatus(db))
	// S124-8 (N7): dismiss flips organizations.onboarding_dismissed — an ORG-WIDE
	// write, so it must require a writer role (a Viewer could otherwise dismiss the
	// onboarding for the whole org).
	g.POST("/dismiss", Dismiss(db), auth.RequireRole("Admin", "SecurityAnalyst"))
	// S89-5: guided "first 30 days" path (7 data-derived steps).
	g.GET("/progress", GetProgressHandler(db))
}
