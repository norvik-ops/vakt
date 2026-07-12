// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package comments

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// Register wires the shared comments routes under the supplied authenticated group.
//
// Routes registered:
//
//	GET    /comments?entity_type=<type>&entity_id=<uuid>
//	POST   /comments
//	DELETE /comments/:id
//	GET    /settings/team/members
func Register(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(NewRepository(db), db)
	// S124-8 (N7): writing a comment is a collaboration write — the read-only
	// roles (Viewer, AuditorReadOnly) must not create/delete comments. Delete is
	// additionally author-ownership-checked in the repository (non-authors get 403
	// unless Admin), so this gate only bounds who may reach the write path.
	collab := auth.RequireRole("Admin", "SecurityAnalyst", "InternalAuditor")
	g.GET("/comments", h.ListComments)
	g.POST("/comments", h.CreateComment, collab)
	g.DELETE("/comments/:id", h.DeleteComment, collab)
	g.GET("/settings/team/members", h.ListTeamMembers)
}
