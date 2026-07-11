// Package retention registers HTTP routes for the data-retention configuration API.
package retention

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register mounts retention-config routes under the provided Echo group.
// The caller passes the `protected` group (/api/v1 with auth/CSRF/MFA/OrgRL);
// routes are placed under /retention.
//
// S121-B5 (R5): the pruning-schedule PUT was previously mounted on the bare
// `api` group with only inline auth — no CSRF and no role gate, so a Viewer
// could shorten evidence-retention windows and trigger early deletion (masked
// only by demo.Guard in demo mode; open in prod with demo:false). Mounting on
// `protected` restores CSRF; UpdateConfig is gated to Admin.
func Register(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	admin := auth.RequireRole("Admin")
	r := g.Group("/retention")
	r.GET("/config", h.GetConfig)
	r.PUT("/config", h.UpdateConfig, admin)
}
