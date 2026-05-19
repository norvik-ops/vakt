// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// registerAccessReviewRoutes wires access review routes under the provided group.
func registerAccessReviewRoutes(g *echo.Group, h *Handler) {
	ar := auth.RequireRole("Admin", "SecurityAnalyst")
	admin := auth.RequireRole("Admin")
	g.GET("/access-reviews", h.ListAccessReviewCampaigns, ar)
	g.POST("/access-reviews", h.CreateAccessReviewCampaign, admin)
	g.GET("/access-reviews/:id", h.GetAccessReviewCampaign, ar)
	g.PUT("/access-reviews/:id", h.UpdateAccessReviewCampaign, admin)
	g.DELETE("/access-reviews/:id", h.DeleteAccessReviewCampaign, admin)
	g.GET("/access-reviews/:id/items", h.ListAccessReviewItems, ar)
	g.POST("/access-reviews/:id/items", h.CreateAccessReviewItem, ar)
	g.PUT("/access-reviews/:id/items/:itemId", h.UpdateAccessReviewItem, ar)
}
