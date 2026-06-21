// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	bsi "github.com/matharnica/vakt/internal/modules/vaktcomply/bsi"
)

// GetBSIModelingMatrix handles GET /api/v1/vaktcomply/bsi-modeling.
func (h *Handler) GetBSIModelingMatrix(c echo.Context) error {
	entries, err := h.service.BSI.GetBSIModelingMatrix(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi modeling matrix")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI modeling matrix", "CK_BSI_MATRIX_FAILED")
	}
	if entries == nil {
		entries = []bsi.BSIModelingEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

// CreateBSIModeling handles POST /api/v1/vaktcomply/bsi-modeling.
func (h *Handler) CreateBSIModeling(c echo.Context) error {
	var in bsi.CreateBSIModelingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	entry, err := h.service.BSI.CreateBSIModeling(c.Request().Context(), orgID(c), userID(c), in)
	if err != nil {
		if strings.Contains(err.Error(), "mapping already exists") {
			return errResp(c, http.StatusConflict, "A mapping for this asset and control already exists", "CK_BSI_DUPLICATE")
		}
		log.Error().Err(err).Msg("create bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to create BSI modeling entry", "CK_BSI_CREATE_FAILED")
	}
	return c.JSON(http.StatusCreated, entry)
}

// UpdateBSIModeling handles PATCH /api/v1/vaktcomply/bsi-modeling/:id.
func (h *Handler) UpdateBSIModeling(c echo.Context) error {
	id := c.Param("id")
	var in bsi.UpdateBSIModelingInput
	if err := c.Bind(&in); err != nil {
		return errResp(c, http.StatusBadRequest, "invalid request body", "CK_BAD_REQUEST")
	}
	if err := h.validate.Struct(in); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "Ungültige Eingabe", "code": "VALIDATION_ERROR"})
	}
	entry, err := h.service.BSI.UpdateBSIModeling(c.Request().Context(), orgID(c), id, in)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "BSI modeling entry not found", "CK_BSI_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("update bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to update BSI modeling entry", "CK_BSI_UPDATE_FAILED")
	}
	return c.JSON(http.StatusOK, entry)
}

// DeleteBSIModeling handles DELETE /api/v1/vaktcomply/bsi-modeling/:id.
func (h *Handler) DeleteBSIModeling(c echo.Context) error {
	id := c.Param("id")
	if err := h.service.BSI.DeleteBSIModeling(c.Request().Context(), orgID(c), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errResp(c, http.StatusNotFound, "BSI modeling entry not found", "CK_BSI_NOT_FOUND")
		}
		log.Error().Err(err).Str("id", id).Msg("delete bsi modeling")
		return errResp(c, http.StatusInternalServerError, "failed to delete BSI modeling entry", "CK_BSI_DELETE_FAILED")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetBSIBausteinSuggestions handles GET /api/v1/vaktcomply/bsi-modeling/suggestions.
// Query param: ?asset_type=server
func (h *Handler) GetBSIBausteinSuggestions(c echo.Context) error {
	assetType := c.QueryParam("asset_type")
	suggestions := h.service.BSI.GetSuggestedBausteine(assetType)
	return c.JSON(http.StatusOK, map[string][]string{"suggestions": suggestions})
}

// ExportBSIModelingPDF handles GET /api/v1/vaktcomply/bsi-modeling/export-pdf.
// Not yet implemented — returns 501.
func (h *Handler) ExportBSIModelingPDF(c echo.Context) error {
	return errResp(c, http.StatusNotImplemented, "PDF export not yet implemented", "CK_BSI_PDF_NOT_IMPLEMENTED")
}

// ExportBSIModelingXLSX handles GET /api/v1/vaktcomply/bsi-modeling/export-xlsx.
// Not yet implemented — returns 501.
func (h *Handler) ExportBSIModelingXLSX(c echo.Context) error {
	return errResp(c, http.StatusNotImplemented, "XLSX export not yet implemented", "CK_BSI_XLSX_NOT_IMPLEMENTED")
}

// GetBSIModelingStats handles GET /api/v1/vaktcomply/bsi-modeling/stats.
func (h *Handler) GetBSIModelingStats(c echo.Context) error {
	stats, err := h.service.BSI.GetBSIModelingStats(c.Request().Context(), orgID(c))
	if err != nil {
		log.Error().Err(err).Msg("get bsi modeling stats")
		return errResp(c, http.StatusInternalServerError, "failed to get BSI modeling stats", "CK_BSI_STATS_FAILED")
	}
	return c.JSON(http.StatusOK, stats)
}
