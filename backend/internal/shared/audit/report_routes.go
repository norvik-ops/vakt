// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// RegisterRoutes mounts GET /audit-report on the provided echo group.
// The group must already have auth middleware applied.
// Accessible by Admin, SecurityAnalyst, Viewer, and AuditorReadOnly roles.
// Requires the FeatureAuditPDF Pro license feature.
func RegisterReport(g *echo.Group, db *pgxpool.Pool, readiness dashboard.ReadinessSource) {
	h := NewReportHandler(db, readiness)
	ro := auth.RequireRole("Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly")
	g.GET("/audit-report", h.GenerateAuditReport, ro, features.Require(features.FeatureAuditPDF))
}
