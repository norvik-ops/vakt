package auth

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterSessions mounts session management endpoints.
// Requires auth middleware (already on the group passed in).
func RegisterSessions(g *echo.Group, db *pgxpool.Pool) {
	h := NewSessionHandler(db)
	g.GET("", h.ListSessions)
	g.DELETE("/:id", h.RevokeSession)
	g.DELETE("", h.RevokeAllOtherSessions)
}
