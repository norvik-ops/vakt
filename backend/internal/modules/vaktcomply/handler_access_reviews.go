// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// --- Access Review Campaigns ---

// ListAccessReviewCampaigns handles GET /api/v1/vaktcomply/access-reviews.
func (h *Handler) ListAccessReviewCampaigns(c echo.Context) error {
	campaigns, err := h.service.ListAccessReviewCampaigns(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list access review campaigns")
		return errResp(c, http.StatusInternalServerError, "failed to list access review campaigns", "CK_LIST_ACCESS_REVIEWS_FAILED")
	}
	return c.JSON(http.StatusOK, campaigns)
}

// CreateAccessReviewCampaign handles POST /api/v1/vaktcomply/access-reviews.
func (h *Handler) CreateAccessReviewCampaign(c echo.Context) error {
	var in CreateAccessReviewCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	campaign, err := h.service.CreateAccessReviewCampaign(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to create access review campaign", "CK_CREATE_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusCreated, campaign)
}

// GetAccessReviewCampaign handles GET /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) GetAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	campaign, err := h.service.GetAccessReviewCampaign(c.Request().Context(), orgID(c), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("get access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to get access review campaign", "CK_GET_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, campaign)
}

// UpdateAccessReviewCampaign handles PUT /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) UpdateAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	var in UpdateAccessReviewCampaignInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	campaign, err := h.service.UpdateAccessReviewCampaign(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to update access review campaign", "CK_UPDATE_ACCESS_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, campaign)
}

// DeleteAccessReviewCampaign handles DELETE /api/v1/vaktcomply/access-reviews/:id.
func (h *Handler) DeleteAccessReviewCampaign(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteAccessReviewCampaign(c.Request().Context(), orgID(c), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review campaign not found", "CK_ACCESS_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Msg("delete access review campaign")
		return errResp(c, http.StatusInternalServerError, "failed to delete access review campaign", "CK_DELETE_ACCESS_REVIEW_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Access Review Items ---

// ListAccessReviewItems handles GET /api/v1/vaktcomply/access-reviews/:id/items.
func (h *Handler) ListAccessReviewItems(c echo.Context) error {
	campaignID := c.Param("id")
	items, err := h.service.ListAccessReviewItems(c.Request().Context(), orgID(c), campaignID)
	if err != nil {
		log.Error().Err(err).Str("campaign_id", campaignID).Msg("list access review items")
		return errResp(c, http.StatusInternalServerError, "failed to list access review items", "CK_LIST_ACCESS_REVIEW_ITEMS_FAILED")
	}
	return c.JSON(http.StatusOK, items)
}

// CreateAccessReviewItem handles POST /api/v1/vaktcomply/access-reviews/:id/items.
func (h *Handler) CreateAccessReviewItem(c echo.Context) error {
	campaignID := c.Param("id")
	var in CreateAccessReviewItemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	// Inject campaign_id from path so callers don't have to repeat it in the body
	in.CampaignID = campaignID
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	item, err := h.service.CreateAccessReviewItem(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create access review item")
		return errResp(c, http.StatusInternalServerError, "failed to create access review item", "CK_CREATE_ACCESS_REVIEW_ITEM_FAILED")
	}
	return c.JSON(http.StatusCreated, item)
}

// UpdateAccessReviewItem handles PUT /api/v1/vaktcomply/access-reviews/:id/items/:itemId.
func (h *Handler) UpdateAccessReviewItem(c echo.Context) error {
	itemID := c.Param("itemId")
	var in UpdateAccessReviewItemInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	item, err := h.service.UpdateAccessReviewItem(c.Request().Context(), orgID(c), itemID, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResp(c, http.StatusNotFound, "access review item not found", "CK_ACCESS_REVIEW_ITEM_NOT_FOUND")
		}
		log.Error().Err(err).Msg("update access review item")
		return errResp(c, http.StatusInternalServerError, "failed to update access review item", "CK_UPDATE_ACCESS_REVIEW_ITEM_FAILED")
	}
	return c.JSON(http.StatusOK, item)
}
