// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auditreport

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// RegisterRoutes mounts GET /audit-report on the provided echo group.
// The group must already have auth middleware applied.
// Accessible by Admin, SecurityAnalyst, Viewer, and AuditorReadOnly roles.
// Requires the FeatureAuditPDF Pro license feature.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	h := NewHandler(db)
	ro := auth.RequireRole("Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly")
	g.GET("/audit-report", h.GenerateAuditReport, ro, features.Require(features.FeatureAuditPDF))
}
