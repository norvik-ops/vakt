package search

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the search endpoint on the provided Echo group.
func Register(g *echo.Group, db *pgxpool.Pool, auth echo.MiddlewareFunc) {
	h := NewHandler(db)
	g.GET("/search", h.Search, auth)
}
