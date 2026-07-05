// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apikeys

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register mounts the API key routes under the given protected group.
// All routes require a valid Paseto token (enforced by the caller) and the
// api_access Pro feature.
func Register(g *echo.Group, db *pgxpool.Pool) {
	svc := NewService(db)
	h := NewHandler(svc)

	keys := g.Group("/api-keys", features.Require(features.FeatureAPI))
	// S120-4: key management is a write path — Viewer/AuditorReadOnly/
	// InternalAuditor must not mint keys (a personal key acts with the
	// creator's role, so an ungated create would be privilege escalation).
	writeGate := auth.RequireRole("Admin", "SecurityAnalyst")
	keys.POST("", h.CreateKey, writeGate)
	keys.GET("", h.ListKeys)
	keys.DELETE("/:id", h.RevokeKey, writeGate)
	// Sprint 20 S20-2: Key-Rotation mit 24-h-Grace-Period. Beide Keys
	// (alter + neuer) sind während der Grace gültig — CI-Pipeline kann
	// kontrolliert switchen ohne Down-Time.
	keys.POST("/:id/rotate", h.RotateKey, writeGate)
}
