// Package search registers HTTP routes for the global full-text search API.
package search

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register mounts the global search endpoint on the provided group.
//
// S131-C2 (R-H22): the caller MUST pass the `protected` group so /search inherits
// the org-wide MFA enforcement (MFAEnforceMiddleware). It used to sit on the bare
// `api` group with only AuthMiddleware, which let a user who had not completed the
// org-required TOTP setup search — and read — across every module, bypassing the
// MFA gate that every other authenticated route enforces.
func Register(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	g.GET("/search", h.Search)
}
