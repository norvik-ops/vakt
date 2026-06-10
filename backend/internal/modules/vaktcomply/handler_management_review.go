// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListManagementReviews handles GET /api/v1/vaktcomply/management-reviews.
func (h *Handler) ListManagementReviews(c echo.Context) error {
	reviews, err := h.service.ListManagementReviews(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list management reviews")
		return errResp(c, http.StatusInternalServerError, "failed to list management reviews", "CK_LIST_MGMT_REVIEWS_FAILED")
	}
	if reviews == nil {
		reviews = []ManagementReview{}
	}
	return c.JSON(http.StatusOK, reviews)
}

// CreateManagementReview handles POST /api/v1/vaktcomply/management-reviews.
func (h *Handler) CreateManagementReview(c echo.Context) error {
	var in CreateManagementReviewInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if len(in.ParticipantIDs) == 0 {
		in.ParticipantIDs = json.RawMessage("[]")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	review, err := h.service.CreateManagementReview(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create management review")
		return errResp(c, http.StatusInternalServerError, "failed to create management review", "CK_CREATE_MGMT_REVIEW_FAILED")
	}
	return c.JSON(http.StatusCreated, review)
}

// GetManagementReview handles GET /api/v1/vaktcomply/management-reviews/:id.
func (h *Handler) GetManagementReview(c echo.Context) error {
	id := c.Param("id")
	review, err := h.service.GetManagementReview(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "management review not found", "CK_MGMT_REVIEW_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, review)
}

// UpdateManagementReviewInputs handles PATCH /api/v1/vaktcomply/management-reviews/:id/inputs.
func (h *Handler) UpdateManagementReviewInputs(c echo.Context) error {
	id := c.Param("id")
	var in UpdateManagementReviewInputsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	review, err := h.service.UpdateManagementReviewInputs(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "management review not found", "CK_MGMT_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Str("review_id", id).Msg("update management review inputs")
		return errResp(c, http.StatusInternalServerError, "failed to update management review inputs", "CK_UPDATE_MGMT_REVIEW_INPUTS_FAILED")
	}
	return c.JSON(http.StatusOK, review)
}

// UpdateManagementReviewOutputs handles PATCH /api/v1/vaktcomply/management-reviews/:id/outputs.
func (h *Handler) UpdateManagementReviewOutputs(c echo.Context) error {
	id := c.Param("id")
	var in UpdateManagementReviewOutputsInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	review, err := h.service.UpdateManagementReviewOutputs(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "management review not found", "CK_MGMT_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Str("review_id", id).Msg("update management review outputs")
		return errResp(c, http.StatusInternalServerError, "failed to update management review outputs", "CK_UPDATE_MGMT_REVIEW_OUTPUTS_FAILED")
	}
	return c.JSON(http.StatusOK, review)
}

// ApproveManagementReview handles POST /api/v1/vaktcomply/management-reviews/:id/approve.
func (h *Handler) ApproveManagementReview(c echo.Context) error {
	id := c.Param("id")
	role, _ := c.Get("role").(string)
	review, err := h.service.ApproveManagementReview(c.Request().Context(), orgID(c), id, userID(c), role)
	if err != nil {
		if err.Error() == "only admin can approve" {
			return errResp(c, http.StatusForbidden, "only admin can approve", "CK_MGMT_REVIEW_FORBIDDEN")
		}
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "management review not found", "CK_MGMT_REVIEW_NOT_FOUND")
		}
		log.Error().Err(err).Str("review_id", id).Msg("approve management review")
		return errResp(c, http.StatusInternalServerError, "failed to approve management review", "CK_APPROVE_MGMT_REVIEW_FAILED")
	}
	return c.JSON(http.StatusOK, review)
}

// ExportManagementReviewPDF handles GET /api/v1/vaktcomply/management-reviews/:id/export-pdf.
func (h *Handler) ExportManagementReviewPDF(c echo.Context) error {
	return errResp(c, http.StatusNotImplemented, "PDF export coming soon", "CK_MGMT_REVIEW_PDF_NOT_IMPLEMENTED")
}
