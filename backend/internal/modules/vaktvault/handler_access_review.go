// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S70-5: HTTP handlers for quarterly Vault Access Reviews.

package vaktvault

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListAccessReviews handles GET /api/v1/vaktvault/access-reviews.
func (h *Handler) ListAccessReviews(c echo.Context) error {
	orgID := mustString(c, "org_id")
	reviews, err := h.service.ListAccessReviews(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("list access reviews")
		return serverError(c, err)
	}
	if reviews == nil {
		reviews = []AccessReview{}
	}
	return c.JSON(http.StatusOK, reviews)
}

// CreateAccessReview handles POST /api/v1/vaktvault/access-reviews.
func (h *Handler) CreateAccessReview(c echo.Context) error {
	orgID := mustString(c, "org_id")
	review, err := h.service.CreateAccessReview(c.Request().Context(), orgID)
	if err != nil {
		log.Error().Err(err).Msg("create access review")
		return serverError(c, err)
	}
	return c.JSON(http.StatusCreated, review)
}

// GetAccessReview handles GET /api/v1/vaktvault/access-reviews/:id.
func (h *Handler) GetAccessReview(c echo.Context) error {
	orgID := mustString(c, "org_id")
	review, items, err := h.service.GetAccessReview(c.Request().Context(), orgID, c.Param("id"))
	if err != nil {
		return notFound(c, "access review not found")
	}
	if items == nil {
		items = []AccessReviewItem{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"review": review,
		"items":  items,
	})
}

// CompleteAccessReview handles POST /api/v1/vaktvault/access-reviews/:id/complete.
func (h *Handler) CompleteAccessReview(c echo.Context) error {
	orgID := mustString(c, "org_id")
	reviewerID := mustString(c, "user_id")
	var in CompleteAccessReviewInput
	if err := c.Bind(&in); err != nil {
		return badRequest(c, "invalid request body")
	}
	review, err := h.service.CompleteAccessReview(c.Request().Context(), orgID, c.Param("id"), reviewerID, in.Decisions)
	if err != nil {
		log.Error().Err(err).Msg("complete access review")
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, review)
}
