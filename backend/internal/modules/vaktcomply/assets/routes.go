// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// Register wires the asset inventory into g. cmd/api mounts it twice with the
// same vaktcomply guard: under /vaktcomply (canonical) and under /vaktscan as a
// compatibility alias for clients of the old path. The alias goes away with the
// vaktscan code (docs/stories/module-archivieren.md, S4).
func Register(g *echo.Group, h *Handler) {
	rw := auth.RequireRole("Admin", "SecurityAnalyst")
	ro := auth.RequireRole("Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly")

	assets := g.Group("", ro)
	assets.POST("/assets", h.CreateAsset, rw)
	assets.GET("/assets", h.ListAssets)
	assets.GET("/assets/classification-summary", h.GetClassificationSummary)
	assets.GET("/assets/:id/protection-need", h.GetAssetProtectionNeed)
	assets.GET("/assets/:id", h.GetAsset)
	assets.PUT("/assets/:id", h.UpdateAsset, rw)
	assets.DELETE("/assets/:id", h.DeleteAsset, rw)
	assets.POST("/assets/import", h.ImportAssets, rw)
	// Pro: extended CSV format (name,type,ip,owner,criticality). Still gated by
	// the historic vaktscan_advanced feature; its new licence tier is decided
	// with the licence list in S5.
	assets.POST("/assets/import/csv", h.ImportAssetsCSVExtended, rw, features.Require(features.FeatureSecPulse))
}
