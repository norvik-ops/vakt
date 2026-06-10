// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ListProtectionNeedAssessments handles GET /api/v1/vaktcomply/protection-needs/assessments.
func (h *Handler) ListProtectionNeedAssessments(c echo.Context) error {
	items, err := h.service.ListProtectionNeedAssessments(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("list protection need assessments")
		return errResp(c, http.StatusInternalServerError, "failed to list assessments", "CK_LIST_PNA_FAILED")
	}
	if items == nil {
		items = []ProtectionNeedAssessment{}
	}
	return c.JSON(http.StatusOK, items)
}

// CreateProtectionNeedAssessment handles POST /api/v1/vaktcomply/protection-needs/assessments.
func (h *Handler) CreateProtectionNeedAssessment(c echo.Context) error {
	var in CreateProtectionNeedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pna, err := h.service.CreateProtectionNeedAssessment(c.Request().Context(), orgID(c), in)
	if err != nil {
		log.Error().Err(err).Msg("create protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to create assessment", "CK_CREATE_PNA_FAILED")
	}
	return c.JSON(http.StatusCreated, pna)
}

// GetProtectionNeedAssessment handles GET /api/v1/vaktcomply/protection-needs/assessments/:id.
func (h *Handler) GetProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	pna, err := h.service.GetProtectionNeedAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
	}
	return c.JSON(http.StatusOK, pna)
}

// UpdateProtectionNeedAssessment handles PATCH /api/v1/vaktcomply/protection-needs/assessments/:id.
// Sets C/I/A ratings and recomputes the overall protection need level.
func (h *Handler) UpdateProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	var in UpdateProtectionNeedInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	pna, err := h.service.UpdateProtectionNeedAssessment(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to update assessment", "CK_UPDATE_PNA_FAILED")
	}
	return c.JSON(http.StatusOK, pna)
}

// FinalizeProtectionNeedAssessment handles POST /api/v1/vaktcomply/protection-needs/assessments/:id/finalize.
func (h *Handler) FinalizeProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	pna, err := h.service.FinalizeProtectionNeedAssessment(c.Request().Context(), orgID(c), id)
	if err != nil {
		if isNotFound(err) {
			return errResp(c, http.StatusNotFound, "assessment not found", "CK_PNA_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("finalize protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to finalize assessment", "CK_FINALIZE_PNA_FAILED")
	}
	return c.JSON(http.StatusOK, pna)
}

// DeleteProtectionNeedAssessment handles DELETE /api/v1/vaktcomply/protection-needs/assessments/:id.
func (h *Handler) DeleteProtectionNeedAssessment(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.DeleteProtectionNeedAssessment(c.Request().Context(), orgID(c), id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("delete protection need assessment")
		return errResp(c, http.StatusInternalServerError, "failed to delete assessment", "CK_DELETE_PNA_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// LinkPNAAsset handles PATCH /api/v1/vaktcomply/protection-needs/assessments/:id/asset-link.
// Body: {"vb_asset_id": "uuid"} to link, {"vb_asset_id": null} to unlink.
func (h *Handler) LinkPNAAsset(c echo.Context) error {
	id := c.Param("id")
	var body struct {
		VBAssetID *string `json:"vb_asset_id"`
	}
	if err := c.Bind(&body); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.service.LinkAssetToPNA(c.Request().Context(), orgID(c), id, body.VBAssetID); err != nil {
		log.Error().Err(err).Str("id", id).Msg("link asset to pna")
		return errResp(c, http.StatusInternalServerError, "failed to link asset", "CK_LINK_ASSET_FAILED")
	}
	return c.JSON(http.StatusOK, map[string]any{"pna_id": id, "vb_asset_id": body.VBAssetID})
}
