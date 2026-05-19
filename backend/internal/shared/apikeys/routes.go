// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apikeys

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

// Register mounts the API key routes under the given protected group.
// All routes require a valid Paseto token (enforced by the caller) and the
// api_access Pro feature.
func Register(g *echo.Group, db *pgxpool.Pool) {
	svc := NewService(db)
	h := NewHandler(svc)

	keys := g.Group("/api-keys", license.Require(license.FeatureAPI))
	keys.POST("", h.CreateKey)
	keys.GET("", h.ListKeys)
	keys.DELETE("/:id", h.RevokeKey)
}
