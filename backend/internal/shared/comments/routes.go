// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package comments

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Register wires the shared comments routes under the supplied authenticated group.
//
// Routes registered:
//
//	GET    /comments?entity_type=<type>&entity_id=<uuid>
//	POST   /comments
//	DELETE /comments/:id
func Register(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(NewRepository(db), db)
	g.GET("/comments", h.ListComments)
	g.POST("/comments", h.CreateComment)
	g.DELETE("/comments/:id", h.DeleteComment)
}
