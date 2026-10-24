// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package assets

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
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
	// Extended CSV format (name,type,ip,owner,criticality). Community since
	// 2026-10-23: it was gated by vaktscan_advanced, which left with Vakt Scan,
	// and importing an inventory is a basic ISMS function, not a Pro extra.
	assets.POST("/assets/import/csv", h.ImportAssetsCSVExtended, rw)
}
